package test

import (
	"context"
	"strings"
	"testing"
	"time"

	authpublic "github.com/nikhea/malawi-e-commerce-store/internal/auth/public"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/model"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/service"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/utils"
	userspublic "github.com/nikhea/malawi-e-commerce-store/internal/users/public"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/events"
)

// fakeUsers is an in-memory userspublic.Service.
type fakeUsers struct {
	byID    map[string]record
	byEmail map[string]string
}

type record struct {
	user userspublic.User
	hash string
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{byID: map[string]record{}, byEmail: map[string]string{}}
}

func (f *fakeUsers) Create(_ context.Context, in userspublic.CreateUserInput) (userspublic.User, error) {
	email := strings.ToLower(in.Email)
	if _, taken := f.byEmail[email]; taken {
		return userspublic.User{}, apperr.Conflict("email already registered")
	}
	u := userspublic.User{ID: "user-" + email, Email: email, Name: in.Name, Role: in.Role}
	f.byID[u.ID] = record{user: u, hash: in.PasswordHash}
	f.byEmail[email] = u.ID
	return u, nil
}

func (f *fakeUsers) GetByID(_ context.Context, id string) (userspublic.User, error) {
	r, ok := f.byID[id]
	if !ok {
		return userspublic.User{}, apperr.NotFound("user not found")
	}
	return r.user, nil
}

func (f *fakeUsers) GetByEmail(_ context.Context, email string) (userspublic.User, error) {
	id, ok := f.byEmail[strings.ToLower(email)]
	if !ok {
		return userspublic.User{}, apperr.NotFound("user not found")
	}
	return f.byID[id].user, nil
}

func (f *fakeUsers) GetCredentials(_ context.Context, email string) (userspublic.Credentials, error) {
	id, ok := f.byEmail[strings.ToLower(email)]
	if !ok {
		return userspublic.Credentials{}, apperr.NotFound("user not found")
	}
	r := f.byID[id]
	return userspublic.Credentials{UserID: r.user.ID, Email: r.user.Email, Role: r.user.Role, PasswordHash: r.hash}, nil
}

func (f *fakeUsers) SetRole(_ context.Context, id string, role userspublic.Role) (userspublic.User, error) {
	r, ok := f.byID[id]
	if !ok {
		return userspublic.User{}, apperr.NotFound("user not found")
	}
	r.user.Role = role
	f.byID[id] = r
	return r.user, nil
}

func (f *fakeUsers) SetEmailVerified(_ context.Context, id string, verified bool) (userspublic.User, error) {
	r, ok := f.byID[id]
	if !ok {
		return userspublic.User{}, apperr.NotFound("user not found")
	}
	r.user.EmailVerified = verified
	f.byID[id] = r
	return r.user, nil
}

func (f *fakeUsers) SetPasswordHash(_ context.Context, id string, hash string) (userspublic.User, error) {
	r, ok := f.byID[id]
	if !ok {
		return userspublic.User{}, apperr.NotFound("user not found")
	}
	r.hash = hash
	f.byID[id] = r
	return r.user, nil
}

// fakeAuthRepo is an in-memory token store with the real semantics:
// single-use codes, consumed-token tracking, revocation.
type fakeAuthRepo struct {
	verifications map[string]string // userID → codeHash
	resets        map[string]string // tokenHash → userID
	refresh       map[string]string // tokenHash → userID
	revoked       map[string]bool
	replaced      map[string]string // oldHash → newHash
}

func newFakeAuthRepo() *fakeAuthRepo {
	return &fakeAuthRepo{
		verifications: map[string]string{},
		resets:        map[string]string{},
		refresh:       map[string]string{},
		revoked:       map[string]bool{},
		replaced:      map[string]string{},
	}
}

func (f *fakeAuthRepo) UpsertVerification(_ context.Context, userID, codeHash string, _ time.Time) error {
	f.verifications[userID] = codeHash
	return nil
}

func (f *fakeAuthRepo) ConsumeVerification(_ context.Context, userID, codeHash string) error {
	if f.verifications[userID] != codeHash {
		return apperr.Unauthorized("invalid code")
	}
	delete(f.verifications, userID)
	return nil
}

func (f *fakeAuthRepo) CreateReset(_ context.Context, userID, tokenHash string, _ time.Time) error {
	f.resets[tokenHash] = userID
	return nil
}

func (f *fakeAuthRepo) ConsumeReset(_ context.Context, tokenHash string) (model.PasswordReset, error) {
	userID, ok := f.resets[tokenHash]
	if !ok {
		return model.PasswordReset{}, apperr.NotFound("reset token not found")
	}
	delete(f.resets, tokenHash)
	return model.PasswordReset{UserID: userID, TokenHash: tokenHash}, nil
}

func (f *fakeAuthRepo) CreateRefresh(_ context.Context, userID, tokenHash string, _ time.Time) (string, error) {
	f.refresh[tokenHash] = userID
	return "id-" + tokenHash, nil
}

func (f *fakeAuthRepo) GetRefresh(_ context.Context, tokenHash string) (model.RefreshToken, error) {
	userID, ok := f.refresh[tokenHash]
	if !ok || f.revoked[tokenHash] {
		return model.RefreshToken{}, apperr.NotFound("refresh token not found")
	}
	return model.RefreshToken{ID: "id-" + tokenHash, UserID: userID, TokenHash: tokenHash}, nil
}

func (f *fakeAuthRepo) RotateRefresh(_ context.Context, oldID, newID string) error {
	for hash := range f.refresh {
		if "id-"+hash == oldID {
			f.revoked[hash] = true
			f.replaced[hash] = newID
		}
	}
	return nil
}

func (f *fakeAuthRepo) RevokeRefresh(_ context.Context, tokenHash string) error {
	f.revoked[tokenHash] = true
	return nil
}

func (f *fakeAuthRepo) RevokeUserRefresh(_ context.Context, userID string) error {
	for hash, uid := range f.refresh {
		if uid == userID {
			f.revoked[hash] = true
		}
	}
	return nil
}

func (f *fakeAuthRepo) RefreshUsedElsewhere(_ context.Context, tokenHash string) (model.RefreshToken, bool, error) {
	if f.revoked[tokenHash] {
		if _, ok := f.replaced[tokenHash]; ok {
			return model.RefreshToken{UserID: f.refresh[tokenHash]}, true, nil
		}
	}
	return model.RefreshToken{}, false, nil
}

type fixture struct {
	svc       authpublic.Service
	users     *fakeUsers
	published map[string][]events.Event
}

func newFixture() *fixture {
	fx := &fixture{users: newFakeUsers(), published: map[string][]events.Event{}}
	bus := events.New()
	for _, name := range []string{authpublic.EmailVerificationRequested, authpublic.PasswordResetRequested} {
		name := name
		bus.Subscribe(name, func(_ context.Context, e events.Event) {
			fx.published[name] = append(fx.published[name], e)
		})
	}
	fx.svc = service.NewService(fx.users, newFakeAuthRepo(), bus, service.Config{
		Secret:      "test-secret",
		TTL:         time.Hour,
		AdminEmails: map[string]struct{}{"boss@malawi.mw": {}},
	})
	return fx
}

func testService(users userspublic.Service) authpublic.Service {
	fx := &fixture{users: users.(*fakeUsers), published: map[string][]events.Event{}}
	bus := events.New()
	return service.NewService(fx.users, newFakeAuthRepo(), bus, service.Config{
		Secret:      "test-secret",
		TTL:         time.Hour,
		AdminEmails: map[string]struct{}{"boss@malawi.mw": {}},
	})
}

func TestPasswordRoundtrip(t *testing.T) {
	h, err := utils.Hash("hunter2-secure")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if h == "hunter2-secure" {
		t.Fatal("password stored raw")
	}
	if !utils.Verify(h, "hunter2-secure") {
		t.Fatal("valid password rejected")
	}
	if utils.Verify(h, "wrong") {
		t.Fatal("wrong password accepted")
	}
}

func TestJWTEnforcement(t *testing.T) {
	svc := testService(newFakeUsers())
	pair, err := svc.Register(context.Background(), authpublic.RegisterInput{
		Email: "a@malawi.mw", Name: "A", Password: "password1",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if _, err := svc.Parse(pair.Token); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	for name, tok := range map[string]string{
		"garbage":  "not-a-token",
		"tampered": pair.Token[:len(pair.Token)-2] + "xx",
		"empty":    "",
	} {
		if _, err := svc.Parse(tok); apperr.CodeOf(err) != apperr.CodeUnauthorized {
			t.Fatalf("%s: expected UNAUTHORIZED, got %v", name, err)
		}
	}
}

func TestRegister(t *testing.T) {
	ctx := context.Background()

	t.Run("customer default", func(t *testing.T) {
		pair, err := testService(newFakeUsers()).Register(ctx, authpublic.RegisterInput{
			Email: "shop@malawi.mw", Name: "S", Password: "password1",
		})
		if err != nil {
			t.Fatalf("register: %v", err)
		}
		if pair.Role != string(userspublic.RoleCustomer) || pair.Token == "" {
			t.Fatalf("bad pair: %+v", pair)
		}
	})

	t.Run("admin bootstrap email", func(t *testing.T) {
		pair, err := testService(newFakeUsers()).Register(ctx, authpublic.RegisterInput{
			Email: "boss@malawi.mw", Name: "Boss", Password: "password1",
		})
		if err != nil {
			t.Fatalf("register: %v", err)
		}
		if pair.Role != string(userspublic.RoleAdmin) {
			t.Fatalf("expected admin, got %s", pair.Role)
		}
	})

	t.Run("short password", func(t *testing.T) {
		_, err := testService(newFakeUsers()).Register(ctx, authpublic.RegisterInput{
			Email: "a@malawi.mw", Password: "short",
		})
		if apperr.CodeOf(err) != apperr.CodeValidation {
			t.Fatalf("expected VALIDATION_ERROR, got %v", err)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		svc := testService(newFakeUsers())
		in := authpublic.RegisterInput{Email: "dup@malawi.mw", Password: "password1"}
		if _, err := svc.Register(ctx, in); err != nil {
			t.Fatalf("first register: %v", err)
		}
		if _, err := svc.Register(ctx, in); apperr.CodeOf(err) != apperr.CodeConflict {
			t.Fatalf("expected CONFLICT, got %v", err)
		}
	})
}

func TestLogin(t *testing.T) {
	ctx := context.Background()
	svc := testService(newFakeUsers())
	if _, err := svc.Register(ctx, authpublic.RegisterInput{
		Email: "lu@malawi.mw", Name: "Lu", Password: "password1",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	if _, err := svc.Login(ctx, "lu@malawi.mw", "password1"); err != nil {
		t.Fatalf("valid login rejected: %v", err)
	}
	for name, tc := range map[string][2]string{
		"wrong password": {"lu@malawi.mw", "nope-nope-nope"},
		"unknown email":  {"ghost@malawi.mw", "password1"},
	} {
		if _, err := svc.Login(ctx, tc[0], tc[1]); apperr.CodeOf(err) != apperr.CodeUnauthorized {
			t.Fatalf("%s: expected UNAUTHORIZED, got %v", name, err)
		}
	}
}

func TestVerificationFlow(t *testing.T) {
	ctx := context.Background()
	fx := newFixture()
	if _, err := fx.svc.Register(ctx, authpublic.RegisterInput{
		Email: "v@malawi.mw", Name: "V", Password: "password1",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Register publishes the OTP mail event.
	mails := fx.published[authpublic.EmailVerificationRequested]
	if len(mails) != 1 {
		t.Fatalf("expected 1 verification mail, got %d", len(mails))
	}
	code := mails[0].Payload.(authpublic.VerificationMail).Code
	if len(code) != 6 {
		t.Fatalf("bad OTP format: %q", code)
	}

	// Wrong code rejected.
	if err := fx.svc.VerifyEmail(ctx, "v@malawi.mw", "000000"); apperr.CodeOf(err) != apperr.CodeUnauthorized {
		t.Fatalf("expected UNAUTHORIZED, got %v", err)
	}
	// Right code verifies.
	if err := fx.svc.VerifyEmail(ctx, "v@malawi.mw", code); err != nil {
		t.Fatalf("verify: %v", err)
	}
	u, _ := fx.users.GetByID(ctx, "user-v@malawi.mw")
	if !u.EmailVerified {
		t.Fatal("flag not flipped")
	}
	// Consumed code can't replay.
	if err := fx.svc.VerifyEmail(ctx, "v@malawi.mw", code); err == nil {
		t.Fatal("replayed code accepted")
	}
	// Unknown emails succeed silently (no enumeration).
	if err := fx.svc.RequestVerification(ctx, "ghost@malawi.mw"); err != nil {
		t.Fatalf("unknown email should succeed: %v", err)
	}
}

func TestPasswordResetFlow(t *testing.T) {
	ctx := context.Background()
	fx := newFixture()
	if _, err := fx.svc.Register(ctx, authpublic.RegisterInput{
		Email: "r@malawi.mw", Name: "R", Password: "password1",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := fx.svc.RequestPasswordReset(ctx, "r@malawi.mw"); err != nil {
		t.Fatalf("forgot: %v", err)
	}
	if err := fx.svc.RequestPasswordReset(ctx, "ghost@malawi.mw"); err != nil {
		t.Fatalf("unknown email should succeed: %v", err)
	}
	mails := fx.published[authpublic.PasswordResetRequested]
	if len(mails) != 1 {
		t.Fatalf("expected 1 reset mail, got %d", len(mails))
	}
	token := mails[0].Payload.(authpublic.ResetMail).Token

	if err := fx.svc.ResetPassword(ctx, token, "short"); apperr.CodeOf(err) != apperr.CodeValidation {
		t.Fatalf("expected VALIDATION_ERROR, got %v", err)
	}
	if err := fx.svc.ResetPassword(ctx, token, "password2"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	// Old password dead, new works.
	if _, err := fx.svc.Login(ctx, "r@malawi.mw", "password1"); apperr.CodeOf(err) != apperr.CodeUnauthorized {
		t.Fatalf("old password should fail: %v", err)
	}
	if _, err := fx.svc.Login(ctx, "r@malawi.mw", "password2"); err != nil {
		t.Fatalf("new password should work: %v", err)
	}
	// Token single-use.
	if err := fx.svc.ResetPassword(ctx, token, "password3"); err == nil {
		t.Fatal("reused token accepted")
	}
}

func TestRefreshRotation(t *testing.T) {
	ctx := context.Background()
	fx := newFixture()
	pair, err := fx.svc.Register(ctx, authpublic.RegisterInput{
		Email: "f@malawi.mw", Name: "F", Password: "password1",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if pair.RefreshToken == "" {
		t.Fatal("no refresh token issued")
	}

	// Rotate: new pair, old dies.
	pair2, err := fx.svc.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if pair2.RefreshToken == pair.RefreshToken {
		t.Fatal("token not rotated")
	}
	if _, err := fx.svc.Refresh(ctx, pair.RefreshToken); err == nil {
		t.Fatal("consumed token still works")
	}

	// Reuse of the consumed token = theft: whole chain dies.
	if _, err := fx.svc.Refresh(ctx, pair.RefreshToken); apperr.CodeOf(err) != apperr.CodeUnauthorized {
		t.Fatalf("expected UNAUTHORIZED, got %v", err)
	}
	if _, err := fx.svc.Refresh(ctx, pair2.RefreshToken); err == nil {
		t.Fatal("chain survived theft response")
	}

	// Logout revokes silently, twice is fine.
	fx2 := newFixture()
	p3, err := fx2.svc.Register(ctx, authpublic.RegisterInput{
		Email: "g@malawi.mw", Name: "G", Password: "password1",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := fx2.svc.Logout(ctx, p3.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := fx2.svc.Refresh(ctx, p3.RefreshToken); apperr.CodeOf(err) != apperr.CodeUnauthorized {
		t.Fatalf("logged-out token should fail: %v", err)
	}
	if err := fx2.svc.Logout(ctx, "ghost"); err != nil {
		t.Fatalf("unknown logout should succeed: %v", err)
	}
}
