// SPDX-License-Identifier: MIT
package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVault_SetAndGet(t *testing.T) {
	v, err := New("", "DXRK_VAULT_KEY")
	if err != nil {
		t.Fatal(err)
	}

	if err := v.Set("openai-key", "sk-abc123"); err != nil {
		t.Fatal(err)
	}

	val, ok := v.Get("openai-key")
	if !ok {
		t.Fatal("expected to find key")
	}
	if val != "sk-abc123" {
		t.Fatalf("expected sk-abc123, got %q", val)
	}
}

func TestVault_Get_Missing(t *testing.T) {
	v, err := New("", "")
	if err != nil {
		t.Fatal(err)
	}

	_, ok := v.Get("nonexistent")
	if ok {
		t.Fatal("expected false for missing key")
	}
}

func TestVault_Delete(t *testing.T) {
	v, err := New("", "")
	if err != nil {
		t.Fatal(err)
	}

	_ = v.Set("tmp-key", "tmp-value")
	if err := v.Delete("tmp-key"); err != nil {
		t.Fatal(err)
	}

	_, ok := v.Get("tmp-key")
	if ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestVault_Rotate(t *testing.T) {
	v, err := New("", "")
	if err != nil {
		t.Fatal(err)
	}

	_ = v.Set("api-key", "old-value")
	if err := v.Rotate("api-key", "new-value"); err != nil {
		t.Fatal(err)
	}

	val, ok := v.Get("api-key")
	if !ok {
		t.Fatal("expected key after rotation")
	}
	if val != "new-value" {
		t.Fatalf("expected new-value, got %q", val)
	}
}

func TestVault_Rotate_Missing(t *testing.T) {
	v, err := New("", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := v.Rotate("missing", "value"); err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestVault_List(t *testing.T) {
	v, err := New("", "")
	if err != nil {
		t.Fatal(err)
	}

	_ = v.Set("a", "1")
	_ = v.Set("b", "2")
	_ = v.Set("c", "3")

	names := v.List()
	if len(names) != 3 {
		t.Fatalf("expected 3 secrets, got %d", len(names))
	}
}

func TestVault_Persistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.enc")
	_ = os.Setenv("DXRK_VAULT_KEY_TEST", "test-master-key-12345")
	defer func() { _ = os.Unsetenv("DXRK_VAULT_KEY_TEST") }()

	v1, err := New(path, "DXRK_VAULT_KEY_TEST")
	if err != nil {
		t.Fatal(err)
	}

	if err := v1.Set("test-key", "test-value"); err != nil {
		t.Fatal(err)
	}

	v2, err := New(path, "DXRK_VAULT_KEY_TEST")
	if err != nil {
		t.Fatal(err)
	}

	val, ok := v2.Get("test-key")
	if !ok {
		t.Fatal("expected to find persisted key")
	}
	if val != "test-value" {
		t.Fatalf("expected test-value, got %q", val)
	}
}

func TestVault_BindToEnv(t *testing.T) {
	_ = os.Setenv("MY_API_KEY", "sk-from-env")
	defer func() { _ = os.Unsetenv("MY_API_KEY") }()

	v, err := New("", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := v.BindToEnv("my-key", "MY_API_KEY"); err != nil {
		t.Fatal(err)
	}

	val, ok := v.Get("my-key")
	if !ok {
		t.Fatal("expected to find bound key")
	}
	if val != "sk-from-env" {
		t.Fatalf("expected sk-from-env, got %q", val)
	}
}

func TestVault_Resolve(t *testing.T) {
	_ = os.Setenv("SECRET_VAR", "from-env")
	defer func() { _ = os.Unsetenv("SECRET_VAR") }()

	v, err := New("", "")
	if err != nil {
		t.Fatal(err)
	}

	_ = v.Set("vault-key", "from-vault")

	if val := v.Resolve("vault-key"); val != "from-vault" {
		t.Fatalf("expected from-vault, got %q", val)
	}
	if val := v.Resolve("SECRET_VAR"); val != "from-env" {
		t.Fatalf("expected from-env, got %q", val)
	}
}

func TestVault_Options(t *testing.T) {
	v, err := New("", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := v.Set("rotating-key", "value", WithRotation("weekly"), WithEnvVar("ROTARY_KEY")); err != nil {
		t.Fatal(err)
	}

	val, ok := v.Get("rotating-key")
	if !ok {
		t.Fatal("expected key")
	}
	if val != "value" {
		t.Fatalf("expected value, got %q", val)
	}
}

func TestVault_Encryption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.vault")
	_ = os.Setenv("DXRK_VAULT_KEY2", "master-key-for-testing")
	defer func() { _ = os.Unsetenv("DXRK_VAULT_KEY2") }()

	v, err := New(path, "DXRK_VAULT_KEY2")
	if err != nil {
		t.Fatal(err)
	}
	_ = v.Set("secret", "sensitive-data")
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Fatal("vault file is empty")
	}

	if string(data) == `{"secret":{"value":"sensitive-data"}}` || string(data) == `{"secret":{"value":"sensitive-data","created":"` {
		t.Fatal("vault file is not encrypted")
	}
}

func TestVault_WrongKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wrong.vault")
	_ = os.Setenv("KEY_A", "master-key-a-for-testing")
	_ = os.Setenv("KEY_B", "master-key-b-for-testing")
	defer func() { _ = os.Unsetenv("KEY_A") }()
	defer func() { _ = os.Unsetenv("KEY_B") }()

	v1, err := New(path, "KEY_A")
	if err != nil {
		t.Fatal(err)
	}
	_ = v1.Set("secret", "data")

	_, err = New(path, "KEY_B")
	if err == nil {
		t.Fatal("expected error with wrong master key")
	}
}
