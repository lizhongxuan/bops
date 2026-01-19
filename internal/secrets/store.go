package secrets

import (
	"fmt"
	"os"
)

type Store interface {
	Get(key string) (string, bool)
}

type EnvStore struct {
	Prefix string
}

func (s EnvStore) Get(key string) (string, bool) {
	name := s.Prefix + key
	value, ok := os.LookupEnv(name)
	return value, ok
}

type Injector struct {
	Store Store
}

func (i Injector) Inject(keys []string, vars map[string]any) (map[string]any, error) {
	if i.Store == nil {
		return nil, fmt.Errorf("secret store is nil")
	}
	out := make(map[string]any, len(vars)+1)
	for k, v := range vars {
		out[k] = v
	}

	secrets := map[string]string{}
	for _, key := range keys {
		value, ok := i.Store.Get(key)
		if !ok {
			return nil, fmt.Errorf("secret %q not found", key)
		}
		secrets[key] = value
	}
	if len(secrets) > 0 {
		out["secrets"] = secrets
	}
	return out, nil
}
