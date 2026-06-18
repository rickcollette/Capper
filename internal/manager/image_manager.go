package manager

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"

	"capper/internal/loader"
	"capper/internal/runtime"
	"capper/internal/store"
	"capper/internal/types"
)

type ImageManager struct {
	Store *store.Store
	Debug bool
}

type CreateResult struct {
	Image types.ImageRecord
}

func (m ImageManager) Create(imageName, configPath string) (*CreateResult, error) {
	cfg, err := types.LoadCreateConfig(configPath)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(imageName, ".cap") {
		imageName += ".cap"
	}
	rootfs := cfg.RootFS
	if !filepath.IsAbs(rootfs) {
		rootfs = filepath.Join(filepath.Dir(configPath), rootfs)
	}
	if st, err := os.Stat(rootfs); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("root filesystem directory not found: %s", cfg.RootFS)
	}
	tmp, err := os.MkdirTemp(m.Store.Paths.Tmp, "create-*")
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		// Always clean up on failure; in debug mode also keep tmp on success so
		// the caller can inspect the intermediate artifacts.
		if !success || !m.Debug {
			os.RemoveAll(tmp)
		}
	}()
	rootArchive := filepath.Join(tmp, "rootfs.tar.zst")
	if err := createRootFSArchive(rootfs, rootArchive); err != nil {
		return nil, err
	}
	rootDigest, err := loader.FileDigest(rootArchive)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	manifest := types.CapsuleManifest{
		CapsuleVersion: "0.1",
		Name:           cfg.Name,
		Version:        cfg.Version,
		Created:        now,
		Hostname:       cfg.Hostname,
		Entrypoint:     cfg.Entrypoint,
		Args:           cfg.Args,
		Env:            cfg.Env,
		WorkingDir:     cfg.WorkingDir,
		Shell:          cfg.Shell,
		User:           cfg.User,
		RootFS:         types.RootFSInfo{Archive: "rootfs.tar.zst", Digest: rootDigest, Compression: "zstd"},
		Network:        cfg.Network,
		Resources:      cfg.Resources,
	}
	manifestPath := filepath.Join(tmp, "capsule.json")
	if err := loader.WriteJSON(manifestPath, manifest); err != nil {
		return nil, err
	}
	manifestDigest, err := loader.FileDigest(manifestPath)
	if err != nil {
		return nil, err
	}
	checksums := types.Checksums{
		Algorithm: "sha256",
		Files: map[string]string{
			"capsule.json":   manifestDigest,
			"rootfs.tar.zst": rootDigest,
		},
	}
	if err := loader.WriteJSON(filepath.Join(tmp, "checksums.json"), checksums); err != nil {
		return nil, err
	}
	finalPath := filepath.Join(m.Store.Paths.Images, imageName)
	if err := createCapArchive(tmp, finalPath); err != nil {
		return nil, err
	}
	size, err := fileSize(finalPath)
	if err != nil {
		return nil, err
	}
	digest, err := loader.FileDigest(finalPath)
	if err != nil {
		return nil, err
	}
	id, err := randomHexID()
	if err != nil {
		return nil, err
	}
	img := types.ImageRecord{ID: id, Name: imageName, Version: cfg.Version, Path: finalPath, CreatedAt: now, SizeBytes: size, Digest: digest}
	if err := m.Store.UpsertImage(img); err != nil {
		return nil, err
	}
	success = true
	return &CreateResult{Image: img}, nil
}

func (m ImageManager) List() ([]types.ImageRecord, error) {
	images, err := m.Store.ListImages()
	if err != nil {
		return nil, err
	}
	for i := range images {
		if _, err := os.Stat(images[i].Path); err != nil {
			images[i].Missing = true
		}
	}
	return images, nil
}

func (m ImageManager) Delete(name string) error {
	if !strings.HasSuffix(name, ".cap") {
		name += ".cap"
	}
	running, err := m.runningInstancesForImage(name)
	if err != nil {
		return err
	}
	if len(running) > 0 {
		return fmt.Errorf("image in use by running instances: %s", running[0].Name)
	}
	img, err := m.Store.GetImage(name)
	if err != nil {
		return fmt.Errorf("image not found: %s", name)
	}
	if err := os.Remove(img.Path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return m.Store.DeleteImage(name)
}

func (m ImageManager) runningInstancesForImage(name string) ([]types.Instance, error) {
	instances, err := m.Store.RunningInstancesForImage(name)
	if err != nil {
		return nil, err
	}
	var running []types.Instance
	for _, inst := range instances {
		if runtime.Alive(inst.PID) {
			running = append(running, inst)
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339)
		inst.Status = types.StatusStopped
		inst.StoppedAt = &now
		if err := m.Store.UpdateInstance(inst); err != nil {
			return nil, err
		}
		_ = m.Store.WriteInstanceJSON(inst)
	}
	return running, nil
}

func createRootFSArchive(rootfs, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	enc, err := zstd.NewWriter(out)
	if err != nil {
		return err
	}
	defer enc.Close()
	tw := tar.NewWriter(enc)
	defer tw.Close()
	return filepath.WalkDir(rootfs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == rootfs {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(rootfs, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		var link string
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
			hdr := &tar.Header{
				Typeflag: tar.TypeSymlink,
				Name:     rel,
				Linkname: link,
				Mode:     0o777,
			}
			return tw.WriteHeader(hdr)
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, file); err != nil {
				file.Close()
				return err
			}
			return file.Close()
		}
		return nil
	})
}

func createCapArchive(srcDir, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	tw := tar.NewWriter(out)
	defer tw.Close()
	for _, name := range []string{"capsule.json", "rootfs.tar.zst", "checksums.json"} {
		path := filepath.Join(srcDir, name)
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(tw, file); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

func fileSize(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}
