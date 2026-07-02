package capstart

import (
	"database/sql"
	"encoding/json"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestStore(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := InitSchema(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRecipeStoreCRUD(t *testing.T) {
	db := newTestStore(t)
	store := NewRecipeStore(db)
	recipe := &Recipe{
		Name:        "test-recipe",
		Version:     "1.0.0",
		Title:       "Test Recipe",
		Description: "A recipe used by store tests",
		Category:    "test",
		Tags:        []string{"one", "two"},
		Content:     json.RawMessage(`{"parameters":{"hostname":{"type":"string","required":true}}}`),
	}
	if err := store.CreateRecipe(recipe); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetRecipe(recipe.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != recipe.Name || len(got.Tags) != 2 || got.Checksum == "" {
		t.Fatalf("stored recipe mismatch: %#v", got)
	}
	got.Title = "Updated"
	if err := store.UpdateRecipe(got); err != nil {
		t.Fatal(err)
	}
	list, err := store.ListRecipes("test", nil, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Title != "Updated" {
		t.Fatalf("unexpected list: %#v", list)
	}
	if err := store.DeleteRecipe(recipe.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetRecipe(recipe.ID); err == nil {
		t.Fatal("expected deleted recipe to be hidden")
	}
}

func TestCapStartStatusStores(t *testing.T) {
	db := newTestStore(t)
	recipes := NewRecipeStore(db)
	executions := NewRecipeExecutionStore(db)
	isos := NewISOStore(db)
	jobs := NewInstallationJobStore(db)

	recipe := &Recipe{Name: "queued-app", Version: "1.0.0", Title: "Queued App", Description: "queued", Content: json.RawMessage(`{}`)}
	if err := recipes.CreateRecipe(recipe); err != nil {
		t.Fatal(err)
	}
	exec := &RecipeExecution{RecipeID: recipe.ID, Status: "pending", Config: json.RawMessage(`{}`)}
	if err := executions.CreateExecution(exec); err != nil {
		t.Fatal(err)
	}
	exec.Status = "cancelled"
	if err := executions.UpdateExecution(exec); err != nil {
		t.Fatal(err)
	}
	gotExec, err := executions.GetExecution(exec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotExec.Status != "cancelled" {
		t.Fatalf("execution status = %q", gotExec.Status)
	}

	iso := &ISO{Name: "Debian", Version: "12", OSType: "linux", Architecture: "x86_64"}
	if err := isos.CreateISO(iso); err != nil {
		t.Fatal(err)
	}
	job := &InstallationJob{ISOID: iso.ID, VMID: "vm-1", Status: "pending"}
	if err := jobs.CreateJob(job); err != nil {
		t.Fatal(err)
	}
	job.Status = "cancelled"
	if err := jobs.UpdateJob(job); err != nil {
		t.Fatal(err)
	}
	gotJob, err := jobs.GetJob(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotJob.ISOID != iso.ID || gotJob.Status != "cancelled" {
		t.Fatalf("job mismatch: %#v", gotJob)
	}
}
