package validationenv

import "testing"

func TestValidationEnvStorePutGet(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_, err := store.Put("docker", ValidationEnv{
		Name:  "docker",
		Type:  EnvTypeContainer,
		Image: "bops-agent:latest",
	})
	if err != nil {
		t.Fatalf("put env: %v", err)
	}

	env, _, err := store.Get("docker")
	if err != nil {
		t.Fatalf("get env: %v", err)
	}
	if env.Type != EnvTypeContainer || env.Image == "" {
		t.Fatalf("unexpected env: %+v", env)
	}
}

func TestValidationEnvStoreValidation(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_, err := store.Put("bad", ValidationEnv{
		Name: "bad",
		Type: EnvTypeContainer,
	})
	if err == nil {
		t.Fatalf("expected error for missing image")
	}

	_, err = store.Put("ssh", ValidationEnv{
		Name: "ssh",
		Type: EnvTypeSSH,
	})
	if err == nil {
		t.Fatalf("expected error for missing ssh host")
	}

	_, err = store.Put("agent", ValidationEnv{
		Name: "agent",
		Type: EnvTypeAgent,
	})
	if err == nil {
		t.Fatalf("expected error for missing agent address")
	}
}
