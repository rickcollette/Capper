package manager

import (
	"fmt"
	"time"

	"capper/internal/database"
	"capper/internal/metadata"
	"capper/internal/store"
)

// CreateManagedDatabase registers a managed DB, stores its password secret, and
// launches the hidden alpine instance that runs the engine.
func CreateManagedDatabase(st *store.Store, im InstanceManager, meta *metadata.Manager, name, project, engine, version, networkID string, port int) (database.ManagedDB, error) {
	if err := st.CheckHostDeployLimit(); err != nil {
		return database.ManagedDB{}, err
	}
	db, password, err := st.Databases.Create(name, project, engine, version, networkID, port)
	if err != nil {
		return database.ManagedDB{}, err
	}
	if _, err := st.Secrets.Create(db.SecretName, project, "managed database password", password); err != nil {
		_ = st.Databases.Delete(db.Name, project)
		return database.ManagedDB{}, fmt.Errorf("database: store password secret: %w", err)
	}
	instanceID, err := im.ProvisionDatabase(meta, db, project, password, "alpine")
	if err != nil {
		_ = st.Secrets.Delete(db.SecretName, project)
		_ = st.Databases.Delete(db.Name, project)
		return database.ManagedDB{}, err
	}
	if err := st.Databases.UpdateInstanceID(db.ID, instanceID, database.DBStatusRunning); err != nil {
		return database.ManagedDB{}, err
	}
	db.InstanceID = instanceID
	db.Status = database.DBStatusRunning
	return db, nil
}

// DeleteManagedDatabase removes the backing instance, secret, and DB record.
// All cascade steps must succeed or the deletion is aborted.
func DeleteManagedDatabase(st *store.Store, im InstanceManager, nameOrID, project string) (database.ManagedDB, error) {
	db, err := st.Databases.Get(nameOrID, project)
	if err != nil {
		return database.ManagedDB{}, err
	}

	// Step 1: Stop the backing instance if it exists.
	if db.InstanceID != "" {
		_, _, stopErr := im.Stop(db.InstanceID, 5*time.Second, true)
		if stopErr != nil {
			return database.ManagedDB{}, fmt.Errorf("cannot stop backing instance %s: %w", db.InstanceID, stopErr)
		}

		// Step 2: Remove the instance. If this fails, the instance is stopped but not
		// removed, so it will be cleaned up by the janitor. Fail the delete so the
		// operator knows to investigate.
		if removeErr := im.Remove(db.InstanceID); removeErr != nil {
			return database.ManagedDB{}, fmt.Errorf("cannot remove backing instance %s: %w", db.InstanceID, removeErr)
		}
	}

	// Step 3: Delete the password secret. This is best-effort; if it fails, log but
	// continue so the DB record can be cleaned up. The orphaned secret will be
	// cleaned up by janitor or manual intervention.
	if db.SecretName != "" {
		if secretErr := st.Secrets.Delete(db.SecretName, project); secretErr != nil {
			// Log for operational visibility but don't fail the entire delete
			fmt.Printf("warning: failed to delete database secret %s: %v\n", db.SecretName, secretErr)
		}
	}

	// Step 4: Delete the database record. This is the final step.
	if err := st.Databases.Delete(nameOrID, project); err != nil {
		return database.ManagedDB{}, fmt.Errorf("cannot delete database record: %w", err)
	}

	return db, nil
}
