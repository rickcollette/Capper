package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"capper/internal/network"
	"capper/internal/types"
	capstore "capper/internal/storage"
)

func (s *Server) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "network:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	nets, err := network.NewManager(s.ctrl.Store.Networks).List(s.project)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, nets, nil)
}

func (s *Server) handleGetNetwork(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "network:inspect", "network/"+name); err != nil {
		writeForbidden(w, err)
		return
	}
	n, leases, err := network.NewManager(s.ctrl.Store.Networks).Inspect(name, s.project)
	if err != nil {
		writeNotFound(w, "network not found")
		return
	}
	writeData(w, map[string]any{"network": n, "leases": leases}, nil)
}

func (s *Server) handleCreateNetwork(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "network:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Name   string            `json:"name"`
		Subnet string            `json:"subnet,omitempty"`
		Mode   string            `json:"mode,omitempty"`
		Labels map[string]string `json:"labels,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	n, err := network.NewManager(s.ctrl.Store.Networks).Create(req.Name, s.project, network.CreateOptions{
		Subnet: req.Subnet,
		Mode:   req.Mode,
		Labels: req.Labels,
	})
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	s.recordEvent(r, "network", n.ID, "network.created", map[string]any{"name": req.Name})
	writeJSON(w, http.StatusCreated, Envelope{Data: n})
}

func (s *Server) handleDeleteNetwork(w http.ResponseWriter, r *http.Request) {
	nameOrID := r.PathValue("name")
	n, err := s.ctrl.Store.Networks.Get(nameOrID, s.project)
	if err != nil {
		writeNotFound(w, "network not found")
		return
	}
	if err := s.authorize(r, "network:delete", "network/"+n.Name); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := network.NewManager(s.ctrl.Store.Networks).Delete(n.Name, s.project); err != nil {
		writeBadRequest(w, err)
		return
	}
	s.recordEvent(r, "network", n.ID, "network.deleted", map[string]any{"name": n.Name})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAttachNetwork(w http.ResponseWriter, r *http.Request) {
	networkName := r.PathValue("name")
	instanceRef := r.PathValue("instance")
	if err := s.authorize(r, "network:attach", "network/"+networkName); err != nil {
		writeForbidden(w, err)
		return
	}
	inst, err := s.ctrl.Store.ResolveInstance(instanceRef)
	if err != nil {
		writeNotFound(w, "instance not found")
		return
	}
	if inst.NetworkID != "" {
		writeBadRequest(w, fmt.Errorf("instance already attached to a network; detach first"))
		return
	}
	netMgr := network.NewManager(s.ctrl.Store.Networks)
	lease, err := netMgr.Connect(inst.ID, networkName, s.project, "")
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	n, _, _ := netMgr.Inspect(networkName, s.project)
	hostVeth, instanceVeth := network.VethNames(inst.ID)
	if err := network.CreateVeth(n.Bridge, hostVeth, instanceVeth); err != nil {
		_ = netMgr.Disconnect(inst.ID, networkName, s.project)
		writeBadRequest(w, err)
		return
	}
	prefix := network.SubnetPrefix(n.Subnet)
	if inst.Status == types.StatusRunning && inst.PID > 0 {
		if err := network.HotAttachNetNS(inst.PID, instanceVeth, lease.IP, prefix, n.Gateway); err != nil {
			_ = network.DeleteVeth(hostVeth)
			_ = netMgr.Disconnect(inst.ID, networkName, s.project)
			writeBadRequest(w, err)
			return
		}
	} else {
		if err := network.SetupInstanceNetNS(inst.ID, instanceVeth, lease.IP, prefix, n.Gateway); err != nil {
			_ = network.DeleteVeth(hostVeth)
			_ = netMgr.Disconnect(inst.ID, networkName, s.project)
			writeBadRequest(w, err)
			return
		}
	}
	inst.NetworkID = n.ID
	inst.NetworkIP = lease.IP
	if err := s.ctrl.Store.UpdateInstance(*inst); err != nil {
		writeInternal(w, err)
		return
	}
	_ = s.ctrl.Store.WriteInstanceJSON(*inst)
	writeData(w, map[string]any{"status": "attached", "ip": lease.IP, "hot": inst.Status == types.StatusRunning}, nil)
}

func (s *Server) handleDetachNetwork(w http.ResponseWriter, r *http.Request) {
	networkName := r.PathValue("name")
	instanceRef := r.PathValue("instance")
	if err := s.authorize(r, "network:detach", "network/"+networkName); err != nil {
		writeForbidden(w, err)
		return
	}
	inst, err := s.ctrl.Store.ResolveInstance(instanceRef)
	if err != nil {
		writeNotFound(w, "instance not found")
		return
	}
	netMgr := network.NewManager(s.ctrl.Store.Networks)
	if err := netMgr.Disconnect(inst.ID, networkName, s.project); err != nil {
		writeBadRequest(w, err)
		return
	}
	hostVeth, _ := network.VethNames(inst.ID)
	_ = network.DeleteVeth(hostVeth) // deletes both ends; also removes instance's veth if hot-attached
	if inst.Status != types.StatusRunning {
		_ = network.TeardownInstanceNetNS(inst.ID)
	}
	inst.NetworkID = ""
	inst.NetworkIP = ""
	if err := s.ctrl.Store.UpdateInstance(*inst); err != nil {
		writeInternal(w, err)
		return
	}
	_ = s.ctrl.Store.WriteInstanceJSON(*inst)
	writeData(w, map[string]any{"status": "detached"}, nil)
}

func (s *Server) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "storage:bucket:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	buckets, err := s.storage.ListBuckets()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, buckets, nil)
}

func (s *Server) handleGetBucket(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if err := s.authorize(r, "storage:bucket:inspect", "bucket/"+bucket); err != nil {
		writeForbidden(w, err)
		return
	}
	b, err := s.storage.GetBucket(bucket)
	if err != nil {
		writeNotFound(w, "bucket not found")
		return
	}
	writeData(w, b, nil)
}

func (s *Server) handleCreateBucket(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "storage:bucket:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Name       string `json:"name"`
		Versioning bool   `json:"versioning"`
		QuotaBytes int64  `json:"quotaBytes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	b, err := s.storage.CreateBucket(capstore.CreateBucketOptions{
		Name:       req.Name,
		Versioning: req.Versioning,
		QuotaBytes: req.QuotaBytes,
	})
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: b})
}

func (s *Server) handleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if err := s.authorize(r, "storage:bucket:delete", "bucket/"+bucket); err != nil {
		writeForbidden(w, err)
		return
	}
	force := r.URL.Query().Get("force") == "true"
	if err := s.storage.DeleteBucket(bucket, force); err != nil {
		writeBadRequest(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListObjects(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if err := s.authorize(r, "storage:object:list", "bucket/"+bucket); err != nil {
		writeForbidden(w, err)
		return
	}
	prefix := r.URL.Query().Get("prefix")
	objects, err := s.storage.ListObjects(bucket, prefix)
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	writeData(w, objects, nil)
}

func (s *Server) handleDeleteObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")
	if err := s.authorize(r, "storage:object:delete", "bucket/"+bucket); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.storage.DeleteObject(bucket, key); err != nil {
		writeBadRequest(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")
	if err := s.authorize(r, "storage:object:get", "bucket/"+bucket); err != nil {
		writeForbidden(w, err)
		return
	}
	mgr := s.storage
	meta, rc, err := mgr.ObjectService().GetObject(bucket, key)
	if err != nil {
		writeNotFound(w, "object not found")
		return
	}
	defer rc.Close()
	if meta.ContentType != "" {
		w.Header().Set("Content-Type", meta.ContentType)
	}
	w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
	_, _ = io.Copy(w, rc)
}

func (s *Server) handlePutObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")
	if err := s.authorize(r, "storage:object:put", "bucket/"+bucket); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		writeBadRequest(w, err)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	defer file.Close()
	tmp, err := os.CreateTemp(s.ctrl.Store.Paths.Tmp, "obj-*")
	if err != nil {
		writeInternal(w, err)
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		writeInternal(w, err)
		return
	}
	if err := tmp.Close(); err != nil {
		writeInternal(w, err)
		return
	}
	obj, err := s.storage.PutObject(bucket, key, tmpPath)
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: obj})
}

func (s *Server) handlePutObjectRaw(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")
	if err := s.authorize(r, "storage:object:put", "bucket/"+bucket); err != nil {
		writeForbidden(w, err)
		return
	}
	tmp, err := os.CreateTemp(s.ctrl.Store.Paths.Tmp, "obj-*")
	if err != nil {
		writeInternal(w, err)
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, r.Body); err != nil {
		tmp.Close()
		writeInternal(w, err)
		return
	}
	if err := tmp.Close(); err != nil {
		writeInternal(w, err)
		return
	}
	obj, err := s.storage.PutObject(bucket, key, tmpPath)
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: obj})
}

func (s *Server) handleListVolumes(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "storage:volume:list", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	vols, err := s.storage.ListVolumes()
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeData(w, vols, nil)
}

func (s *Server) handleCreateVolume(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r, "storage:volume:create", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		Name      string `json:"name"`
		SizeBytes int64  `json:"sizeBytes"`
		Class     string `json:"class,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	v, err := s.storage.CreateVolume(capstore.CreateVolumeOptions{
		Name:      req.Name,
		SizeBytes: req.SizeBytes,
		Class:     req.Class,
	})
	if err != nil {
		writeBadRequest(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, Envelope{Data: v})
}

func (s *Server) handleAttachVolume(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "storage:volume:attach", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	var req struct {
		InstanceID string `json:"instanceId"`
		MountPath  string `json:"mountPath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if err := s.storage.AttachVolume(name, req.InstanceID, req.MountPath); err != nil {
		writeBadRequest(w, err)
		return
	}
	writeData(w, map[string]string{"status": "attached"}, nil)
}

func (s *Server) handleDetachVolume(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "storage:volume:detach", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.storage.DetachVolume(name); err != nil {
		writeBadRequest(w, err)
		return
	}
	writeData(w, map[string]string{"status": "detached"}, nil)
}

func (s *Server) handleDeleteVolume(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.authorize(r, "storage:volume:delete", "project:"+s.project); err != nil {
		writeForbidden(w, err)
		return
	}
	if err := s.storage.DeleteVolume(name); err != nil {
		writeBadRequest(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
