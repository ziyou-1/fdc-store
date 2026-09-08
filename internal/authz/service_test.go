package authz

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAuthzServiceTest(t *testing.T) *Service {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	svc, err := NewService(db)
	if err != nil {
		t.Fatalf("new authz service failed: %v", err)
	}
	return svc
}

func TestEnforceAdminWithRolePolicy(t *testing.T) {
	svc := setupAuthzServiceTest(t)
	if err := svc.GrantRolePolicy("ops", "/admin/products/:id", "GET"); err != nil {
		t.Fatalf("grant role policy failed: %v", err)
	}
	if err := svc.SetAdminRoles(1, []string{"ops"}); err != nil {
		t.Fatalf("set admin roles failed: %v", err)
	}

	allow, err := svc.EnforceAdmin(1, "/api/v1/admin/products/42", "get")
	if err != nil {
		t.Fatalf("enforce allow failed: %v", err)
	}
	if !allow {
		t.Fatalf("expected allow=true")
	}

	allow, err = svc.EnforceAdmin(1, "/api/v1/admin/products/42", "POST")
	if err != nil {
		t.Fatalf("enforce deny failed: %v", err)
	}
	if allow {
		t.Fatalf("expected allow=false")
	}
}

func TestSetAdminRolesOverride(t *testing.T) {
	svc := setupAuthzServiceTest(t)
	if err := svc.GrantRolePolicy("ops", "/admin/orders", "GET"); err != nil {
		t.Fatalf("grant ops policy failed: %v", err)
	}
	if err := svc.GrantRolePolicy("billing_custom", "/admin/payments", "GET"); err != nil {
		t.Fatalf("grant billing policy failed: %v", err)
	}

	if err := svc.SetAdminRoles(2, []string{"ops"}); err != nil {
		t.Fatalf("set first role failed: %v", err)
	}
	roles, err := svc.GetAdminRoles(2)
	if err != nil {
		t.Fatalf("get roles failed: %v", err)
	}
	if len(roles) != 1 || roles[0] != "role:ops" {
		t.Fatalf("roles want [role:ops], got=%v", roles)
	}

	if err := svc.SetAdminRoles(2, []string{"billing_custom"}); err != nil {
		t.Fatalf("set second role failed: %v", err)
	}
	roles, err = svc.GetAdminRoles(2)
	if err != nil {
		t.Fatalf("get roles failed: %v", err)
	}
	if len(roles) != 1 || roles[0] != "role:billing_custom" {
		t.Fatalf("roles want [role:billing_custom], got=%v", roles)
	}

	allow, err := svc.EnforceAdmin(2, "/admin/orders", "GET")
	if err != nil {
		t.Fatalf("enforce old role failed: %v", err)
	}
	if allow {
		t.Fatalf("expected old role permission removed")
	}

	allow, err = svc.EnforceAdmin(2, "/admin/payments", "GET")
	if err != nil {
		t.Fatalf("enforce new role failed: %v", err)
	}
	if !allow {
		t.Fatalf("expected new role permission granted")
	}
}

func TestNormalizeObject(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "/api/v1/admin/orders/:id", want: "/admin/orders/:id"},
		{in: "/admin/orders/:id", want: "/admin/orders/:id"},
		{in: "admin/orders", want: "/admin/orders"},
		{in: "/api/v1", want: "/"},
		{in: "", want: "/"},
	}
	for _, item := range cases {
		got := NormalizeObject(item.in)
		if got != item.want {
			t.Fatalf("normalize object failed, in=%q want=%q got=%q", item.in, item.want, got)
		}
	}
}

func TestBootstrapBuiltinRoles(t *testing.T) {
	svc := setupAuthzServiceTest(t)
	if err := svc.BootstrapBuiltinRoles(); err != nil {
		t.Fatalf("bootstrap builtin roles failed: %v", err)
	}

	roles, err := svc.ListRoles()
	if err != nil {
		t.Fatalf("list roles failed: %v", err)
	}
	wantRoles := map[string]bool{
		"role:readonly_auditor": true,
		"role:operations":       true,
		"role:support":          true,
		"role:finance":          true,
	}
	for _, role := range roles {
		delete(wantRoles, role)
	}
	if len(wantRoles) != 0 {
		t.Fatalf("builtin roles missing: %v", wantRoles)
	}

	if err := svc.SetAdminRoles(3, []string{"operations"}); err != nil {
		t.Fatalf("set admin roles failed: %v", err)
	}

	allow, err := svc.EnforceAdmin(3, "/admin/dashboard/overview", "GET")
	if err != nil {
		t.Fatalf("enforce inherited readonly failed: %v", err)
	}
	if !allow {
		t.Fatalf("expected inherited readonly permission")
	}

	allow, err = svc.EnforceAdmin(3, "/admin/settings", "PUT")
	if err != nil {
		t.Fatalf("enforce readonly write failed: %v", err)
	}
	if allow {
		t.Fatalf("expected readonly inherited role deny write")
	}

	if err := svc.SetAdminRoles(4, []string{"system_admin"}); err != nil {
		t.Fatalf("set system admin role failed: %v", err)
	}
	for _, action := range []string{"GET", "PUT"} {
		allow, err = svc.EnforceAdmin(4, "/admin/settings/google-auth", action)
		if err != nil {
			t.Fatalf("enforce google auth settings %s failed: %v", action, err)
		}
		if !allow {
			t.Fatalf("expected system admin to access google auth settings with %s", action)
		}
	}

	for _, roleCase := range []struct {
		adminID uint
		role    string
		want    bool
	}{
		{adminID: 5, role: "support", want: true},
		{adminID: 6, role: "system_admin", want: true},
		{adminID: 7, role: "readonly_auditor", want: false},
	} {
		if err := svc.SetAdminRoles(roleCase.adminID, []string{roleCase.role}); err != nil {
			t.Fatalf("assign %s role failed: %v", roleCase.role, err)
		}
		allow, err = svc.EnforceAdmin(roleCase.adminID, "/admin/users/42/oauth/google", "DELETE")
		if err != nil {
			t.Fatalf("enforce google unbind for %s failed: %v", roleCase.role, err)
		}
		if allow != roleCase.want {
			t.Fatalf("google unbind for %s allow=%v, want %v", roleCase.role, allow, roleCase.want)
		}
	}
}

// 启动播种不得覆盖管理员对内置角色做的修改。
//
// 原先的实现每次启动都把内置角色「补齐种子里的、删掉种子外的」，等于把它们焊死：
// 后台改完一重启就被打回原样，且不报错、无痕迹。放开编辑后，这条必须成立，
// 否则「可以改」只是假象。
func TestBootstrapDoesNotClobberBuiltinRoleEdits(t *testing.T) {
	svc := setupAuthzServiceTest(t)
	if err := svc.BootstrapBuiltinRoles(); err != nil {
		t.Fatalf("bootstrap builtin roles failed: %v", err)
	}

	// 管理员给内置角色加一条策略、删一条默认策略
	if err := svc.GrantRolePolicy("readonly_auditor", "/admin/payment-channels", "GET"); err != nil {
		t.Fatalf("grant policy on builtin role failed: %v", err)
	}
	if err := svc.RevokeRolePolicy("readonly_auditor", "/admin/dashboard/overview", "GET"); err != nil {
		t.Fatalf("revoke policy on builtin role failed: %v", err)
	}

	// 再次启动
	if err := svc.BootstrapBuiltinRoles(); err != nil {
		t.Fatalf("second bootstrap failed: %v", err)
	}

	policies, err := svc.GetRolePolicies("readonly_auditor")
	if err != nil {
		t.Fatalf("get policies failed: %v", err)
	}
	hasAdded, hasRemoved := false, false
	for _, policy := range policies {
		if policy.Object == "/admin/payment-channels" && policy.Action == "GET" {
			hasAdded = true
		}
		if policy.Object == "/admin/dashboard/overview" && policy.Action == "GET" {
			hasRemoved = true
		}
	}
	if !hasAdded {
		t.Fatal("重启后管理员新增的策略被抹掉了")
	}
	if hasRemoved {
		t.Fatal("重启后管理员删掉的策略被种子加了回来")
	}
}

// 内置角色的策略允许增删，但角色本身不允许删除——
// 删掉角色会让已绑定它的管理员凭空失去全部权限，且没有入口把角色建回来。
func TestBuiltinRolePoliciesEditableButRoleNotDeletable(t *testing.T) {
	svc := setupAuthzServiceTest(t)
	if err := svc.BootstrapBuiltinRoles(); err != nil {
		t.Fatalf("bootstrap builtin roles failed: %v", err)
	}

	for _, role := range []string{"readonly_auditor", "operations", "support", "integration", "finance", "system_admin"} {
		if err := svc.GrantRolePolicy(role, "/admin/products", "GET"); err != nil {
			t.Fatalf("角色 %s 应当允许增加策略，实得: %v", role, err)
		}
		if err := svc.RevokeRolePolicy(role, "/admin/products", "GET"); err != nil {
			t.Fatalf("角色 %s 应当允许删除策略，实得: %v", role, err)
		}
		if err := svc.DeleteRole(role); !errors.Is(err, ErrImmutableBuiltinRole) {
			t.Fatalf("角色 %s 不该被删除，实得: %v", role, err)
		}
	}
}

// 把某个内置角色的策略全部删光后重启，应恢复默认策略。
// 这是万一把自己锁在后台外面时的唯一退路，不需要登服务器改库。
func TestBootstrapReseedsBuiltinRoleWithNoPolicies(t *testing.T) {
	svc := setupAuthzServiceTest(t)
	if err := svc.BootstrapBuiltinRoles(); err != nil {
		t.Fatalf("bootstrap builtin roles failed: %v", err)
	}

	before, err := svc.GetRolePolicies("finance")
	if err != nil {
		t.Fatalf("get policies failed: %v", err)
	}
	if len(before) == 0 {
		t.Fatal("finance 角色首次播种后应当有策略")
	}
	for _, policy := range before {
		if err := svc.RevokeRolePolicy("finance", policy.Object, policy.Action); err != nil {
			t.Fatalf("revoke failed: %v", err)
		}
	}
	if mid, err := svc.GetRolePolicies("finance"); err != nil || len(mid) != 0 {
		t.Fatalf("策略应已删光，实得 %d 条 err=%v", len(mid), err)
	}

	if err := svc.BootstrapBuiltinRoles(); err != nil {
		t.Fatalf("re-bootstrap failed: %v", err)
	}
	after, err := svc.GetRolePolicies("finance")
	if err != nil {
		t.Fatalf("get policies failed: %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("删光后重启应恢复默认策略：恢复 %d 条，原有 %d 条", len(after), len(before))
	}
}
