package service

import "testing"

func TestAuthCredentialsRequireEmail(t *testing.T) {
	t.Parallel()

	if _, _, err := normalizeAuthCredentials(AuthCredentials{
		Email:    "admin",
		Password: "password",
	}); err == nil {
		t.Fatal("normalizeAuthCredentials error = nil, want error")
	}
}

func TestDefaultAdminPasswordHashMatchesPassword(t *testing.T) {
	t.Parallel()

	const hash = "pbkdf2_sha256$210000$YWxhZGluLWRldi1hZG1pbg$iONMwnrln6ivij4VdCYBMDNzlx8nKTrdQQhGssIkXh8"

	if !NewPasswordHasher().Compare(hash, "password") {
		t.Fatal("default admin hash did not match password")
	}
	if NewPasswordHasher().Compare(hash, "wrong-password") {
		t.Fatal("default admin hash matched wrong password")
	}
}
