package api

import (
	"encoding/json"
	"net/http"

	s3server "capper/internal/s3server"
)

func (s *Server) handleGetBucketPolicy(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	p, err := s3server.GetBucketPolicy(s.ctrl.Store.DB, bucket)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, p, nil)
}

func (s *Server) handlePutBucketPolicy(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	var p s3server.BucketPolicy
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid bucket policy JSON: "+err.Error())
		return
	}
	if err := s3server.PutBucketPolicy(s.ctrl.Store.DB, bucket, p); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, p, nil)
}

func (s *Server) handleDeleteBucketPolicy(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if err := s3server.DeleteBucketPolicy(s.ctrl.Store.DB, bucket); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
