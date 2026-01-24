package auth

import (
	"context"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
)

func testDB(t *testing.T) *database.DB {
	t.Helper()
	tmpDir := t.TempDir()

	cfg := &config.DatabaseConfig{
		Path: tmpDir + "/test.db",
	}

	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	t.Cleanup(func() { db.Close() })

	return db
}

func testAuthConfig() *config.AuthConfig {
	return &config.AuthConfig{
		JWT: config.JWTConfig{
			Secret:     "testsecret12345678901234567890123456",
			Issuer:     "test",
			AccessTTL:  15 * time.Minute,
			RefreshTTL: 7 * 24 * time.Hour,
		},
		Password: config.PasswordConfig{
			MinLength: 8,
		},
		AllowRegistration: true,
	}
}

func TestService_CreateUserByAdmin(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	user, err := svc.CreateUserByAdmin(ctx, CreateUserInput{
		Email:    "admin@example.com",
		Password: "password123",
		Role:     RoleAdmin,
		Verified: true,
	})
	if err != nil {
		t.Fatalf("CreateUserByAdmin failed: %v", err)
	}

	if user.Email != "admin@example.com" {
		t.Errorf("Email mismatch: got %s, want admin@example.com", user.Email)
	}
	if user.Role != RoleAdmin {
		t.Errorf("Role mismatch: got %s, want %s", user.Role, RoleAdmin)
	}
	if !user.Verified {
		t.Error("User should be verified")
	}
}

func TestService_CreateUserByAdmin_DefaultRole(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	user, err := svc.CreateUserByAdmin(ctx, CreateUserInput{
		Email:    "user@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("CreateUserByAdmin failed: %v", err)
	}

	if user.Role != RoleUser {
		t.Errorf("Default role should be %s, got %s", RoleUser, user.Role)
	}
}

func TestService_CreateUserByAdmin_InvalidRole(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	_, err := svc.CreateUserByAdmin(ctx, CreateUserInput{
		Email:    "user@example.com",
		Password: "password123",
		Role:     "superadmin",
	})
	if err == nil {
		t.Error("Expected error for invalid role")
	}
}

func TestService_CreateUserByAdmin_DuplicateEmail(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	_, err := svc.CreateUserByAdmin(ctx, CreateUserInput{
		Email:    "user@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("First CreateUserByAdmin failed: %v", err)
	}

	_, err = svc.CreateUserByAdmin(ctx, CreateUserInput{
		Email:    "user@example.com",
		Password: "password456",
	})
	if err == nil {
		t.Error("Expected error for duplicate email")
	}
}

func TestService_ListUsers(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, err := svc.CreateUserByAdmin(ctx, CreateUserInput{
			Email:    "user" + string(rune('a'+i)) + "@example.com",
			Password: "password123",
			Role:     RoleUser,
		})
		if err != nil {
			t.Fatalf("CreateUserByAdmin failed: %v", err)
		}
	}

	result, err := svc.ListUsers(ctx, ListUsersOptions{Limit: 3})
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}

	if result.Total != 5 {
		t.Errorf("Total mismatch: got %d, want 5", result.Total)
	}
	if len(result.Users) != 3 {
		t.Errorf("Users count mismatch: got %d, want 3", len(result.Users))
	}
}

func TestService_ListUsers_Search(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	_, _ = svc.CreateUserByAdmin(ctx, CreateUserInput{Email: "alice@example.com", Password: "password123"})
	_, _ = svc.CreateUserByAdmin(ctx, CreateUserInput{Email: "bob@example.com", Password: "password123"})
	_, _ = svc.CreateUserByAdmin(ctx, CreateUserInput{Email: "charlie@example.com", Password: "password123"})

	result, err := svc.ListUsers(ctx, ListUsersOptions{Search: "alice"})
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("Total mismatch: got %d, want 1", result.Total)
	}
	if len(result.Users) != 1 || result.Users[0].Email != "alice@example.com" {
		t.Error("Expected to find alice@example.com")
	}
}

func TestService_ListUsers_FilterByRole(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	_, _ = svc.CreateUserByAdmin(ctx, CreateUserInput{Email: "user1@example.com", Password: "password123", Role: RoleUser})
	_, _ = svc.CreateUserByAdmin(ctx, CreateUserInput{Email: "user2@example.com", Password: "password123", Role: RoleUser})
	_, _ = svc.CreateUserByAdmin(ctx, CreateUserInput{Email: "admin@example.com", Password: "password123", Role: RoleAdmin})

	result, err := svc.ListUsers(ctx, ListUsersOptions{Role: RoleAdmin})
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("Total mismatch: got %d, want 1", result.Total)
	}
	if len(result.Users) != 1 || result.Users[0].Role != RoleAdmin {
		t.Error("Expected to find admin user only")
	}
}

func TestService_UpdateUser(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	user, err := svc.CreateUserByAdmin(ctx, CreateUserInput{
		Email:    "user@example.com",
		Password: "password123",
		Role:     RoleUser,
	})
	if err != nil {
		t.Fatalf("CreateUserByAdmin failed: %v", err)
	}

	newEmail := "updated@example.com"
	verified := true
	newRole := RoleAdmin

	updated, err := svc.UpdateUser(ctx, user.ID, UpdateUserInput{
		Email:    &newEmail,
		Verified: &verified,
		Role:     &newRole,
	})
	if err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	if updated.Email != newEmail {
		t.Errorf("Email not updated: got %s, want %s", updated.Email, newEmail)
	}
	if !updated.Verified {
		t.Error("Verified not updated")
	}
	if updated.Role != RoleAdmin {
		t.Errorf("Role not updated: got %s, want %s", updated.Role, RoleAdmin)
	}
}

func TestService_UpdateUser_NotFound(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	newEmail := "test@example.com"
	_, err := svc.UpdateUser(ctx, "nonexistent-id", UpdateUserInput{
		Email: &newEmail,
	})
	if err == nil {
		t.Error("Expected error for non-existent user")
	}
}

func TestService_UpdateUser_InvalidRole(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	user, _ := svc.CreateUserByAdmin(ctx, CreateUserInput{
		Email:    "user@example.com",
		Password: "password123",
	})

	invalidRole := "superadmin"
	_, err := svc.UpdateUser(ctx, user.ID, UpdateUserInput{
		Role: &invalidRole,
	})
	if err == nil {
		t.Error("Expected error for invalid role")
	}
}

func TestService_DeleteUser(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	user, err := svc.CreateUserByAdmin(ctx, CreateUserInput{
		Email:    "user@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("CreateUserByAdmin failed: %v", err)
	}

	err = svc.DeleteUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	_, err = svc.GetUserByID(ctx, user.ID)
	if err == nil {
		t.Error("Expected user to be deleted")
	}
}

func TestService_DeleteUser_NotFound(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	err := svc.DeleteUser(ctx, "nonexistent-id")
	if err == nil {
		t.Error("Expected error for non-existent user")
	}
}

func TestService_SetPassword(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	user, err := svc.CreateUserByAdmin(ctx, CreateUserInput{
		Email:    "user@example.com",
		Password: "oldpassword123",
	})
	if err != nil {
		t.Fatalf("CreateUserByAdmin failed: %v", err)
	}

	err = svc.SetPassword(ctx, user.ID, "newpassword456")
	if err != nil {
		t.Fatalf("SetPassword failed: %v", err)
	}

	_, _, err = svc.Login(ctx, LoginInput{
		Email:    "user@example.com",
		Password: "newpassword456",
	}, "", "")
	if err != nil {
		t.Errorf("Login with new password failed: %v", err)
	}
}

func TestService_SetPassword_NotFound(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	err := svc.SetPassword(ctx, "nonexistent-id", "newpassword123")
	if err == nil {
		t.Error("Expected error for non-existent user")
	}
}

func TestService_SetPassword_InvalidPassword(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	user, _ := svc.CreateUserByAdmin(ctx, CreateUserInput{
		Email:    "user@example.com",
		Password: "oldpassword123",
	})

	err := svc.SetPassword(ctx, user.ID, "short")
	if err == nil {
		t.Error("Expected error for short password")
	}
}

func TestService_GetUserByID_WithRole(t *testing.T) {
	db := testDB(t)
	svc := NewService(db, testAuthConfig())

	ctx := context.Background()

	created, _ := svc.CreateUserByAdmin(ctx, CreateUserInput{
		Email:    "admin@example.com",
		Password: "password123",
		Role:     RoleAdmin,
	})

	user, err := svc.GetUserByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}

	if user.Role != RoleAdmin {
		t.Errorf("Role mismatch: got %s, want %s", user.Role, RoleAdmin)
	}
}
