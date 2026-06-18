package controller

import (
	"fmt"

	"capper/internal/certmgr"
	"capper/internal/iam"
	"capper/internal/loader"
	"capper/internal/manager"
	"capper/internal/runtime"
	"capper/internal/store"
)

type Controller struct {
	Images        manager.ImageManager
	Instances     manager.InstanceManager
	Store         *store.Store
	CertMgr       *certmgr.CertManager
	principalType string
	principalID   string
}

func New(st *store.Store, debug bool, runtimeMode string) Controller {
	ld := loader.Loader{Paths: st.Paths, Debug: debug}
	runner := runtime.Runner{Mode: runtimeMode}
	pType, pID := st.IAM.LocalPrincipal()

	certStore := certmgr.NewStore(st.DB)
	_ = certStore.InitSchema()
	cfg := certmgr.DefaultConfig()
	cm := certmgr.NewCertManager(certStore, cfg, st.SecretKey)

	return Controller{
		Images: manager.ImageManager{Store: st, Debug: debug},
		Instances: manager.InstanceManager{
			Store:  st,
			Loader: ld,
			Runner: runner,
		},
		Store:         st,
		CertMgr:       cm,
		principalType: pType,
		principalID:   pID,
	}
}

// Authorize checks whether the current principal may perform action on resource.
// Returns nil if allowed, a descriptive error if denied.
func (c *Controller) Authorize(action, resource string) error {
	if c.Store.IAM == nil {
		return nil
	}
	return c.Store.IAM.Authorize(c.principalType, c.principalID, action, resource)
}

// WhoAmI returns the current principal as "type:id".
func (c *Controller) WhoAmI() string {
	return fmt.Sprintf("%s:%s", c.principalType, c.principalID)
}

// PrincipalType returns the type of the current principal.
func (c *Controller) PrincipalType() string { return c.principalType }

// PrincipalID returns the ID of the current principal.
func (c *Controller) PrincipalID() string { return c.principalID }

// IAM returns the IAM manager.
func (c *Controller) IAM() *iam.Manager { return c.Store.IAM }

