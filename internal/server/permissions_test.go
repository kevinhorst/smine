package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePermissionSettings(t *testing.T) (string, *Server) {
	t.Helper()
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	main := config.NewSettings()
	require.NoError(t, main.SetPermissions(config.Permissions{
		Allow: []string{"Bash(ls *)", "Bash(cat *)"},
		Ask:   []string{"Bash(rm *)"},
		Deny:  []string{"TaskStop"},
	}))
	require.NoError(t, main.Doc().Set([]string{"permissions", "defaultMode"}, json.RawMessage(`"acceptEdits"`)))
	disabled := config.NewSettings()
	require.NoError(t, disabled.SetPermissions(config.Permissions{
		Allow: []string{"Bash(jq *)"},
	}))
	require.NoError(t, config.Save(settingsPath, main, nil))
	require.NoError(t, config.Save(config.DisabledPath(settingsPath), disabled, nil))
	return settingsPath, newTestServer(t, &Options{SettingsPath: settingsPath})
}

func loadPermissions(t *testing.T, settingsPath string) (config.Permissions, config.Permissions) {
	t.Helper()
	main, err := config.Load(settingsPath)
	require.NoError(t, err)
	disabled, err := config.Load(config.DisabledPath(settingsPath))
	require.NoError(t, err)
	mainPerms, err := main.Permissions()
	require.NoError(t, err)
	disabledPerms, err := disabled.Permissions()
	require.NoError(t, err)
	return mainPerms, disabledPerms
}

// permissionItemsHandler serves the add and remove handlers until server.go wires their routes.
func permissionItemsHandler(server *Server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/permissions/{list}/items", server.handleAddPermissionItem)
	mux.HandleFunc("DELETE /api/permissions/{list}/{index}", server.handleRemovePermission)
	return mux
}

func loadDefaultMode(t *testing.T, settingsPath string) string {
	t.Helper()
	main, err := config.Load(settingsPath)
	require.NoError(t, err)
	raw, ok := main.Doc().Get([]string{"permissions", "defaultMode"})
	if !ok {
		return ""
	}
	return string(raw)
}

func TestTogglePermissionNegativeIndex(t *testing.T) {
	settingsPath, server := writePermissionSettings(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/api/permissions/allow/-1/toggle", nil))

	assert.Equal(t, http.StatusBadRequest, response.Code)
	mainPerms, _ := loadPermissions(t, settingsPath)
	assert.Equal(t, []string{"Bash(ls *)", "Bash(cat *)"}, mainPerms.Allow)
}

func TestTogglePermission(t *testing.T) {
	t.Run("allow-to-disabled", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/permissions/allow/0/toggle", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(cat *)"}, mainPerms.Allow)
		assert.Equal(t, []string{"Bash(jq *)", "Bash(ls *)"}, disabledPerms.Allow)
	})

	t.Run("disabled-allow-to-main", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/permissions/disabledAllow/0/toggle", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(ls *)", "Bash(cat *)", "Bash(jq *)"}, mainPerms.Allow)
		assert.Empty(t, disabledPerms.Allow)
	})

	t.Run("ask-to-disabled", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/permissions/ask/0/toggle", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Empty(t, mainPerms.Ask)
		assert.Equal(t, []string{"Bash(rm *)"}, disabledPerms.Ask)
	})

	t.Run("deny-to-disabled", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/permissions/deny/0/toggle", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Empty(t, mainPerms.Deny)
		assert.Equal(t, []string{"TaskStop"}, disabledPerms.Deny)
	})

	t.Run("disabled-deny-to-main", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		parked := httptest.NewRecorder()
		server.Handler().ServeHTTP(parked, formPost("/api/permissions/deny/0/toggle", nil))
		require.Equal(t, 200, parked.Code, parked.Body.String())

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/permissions/disabledDeny/0/toggle", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"TaskStop"}, mainPerms.Deny)
		assert.Empty(t, disabledPerms.Deny)
	})

	t.Run("index-out-of-range-400", func(t *testing.T) {
		_, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/permissions/allow/99/toggle", nil))
		assert.Equal(t, 400, response.Code)
	})

	t.Run("invalid-list-400", func(t *testing.T) {
		_, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/permissions/bogus/0/toggle", nil))
		assert.Equal(t, 400, response.Code)
	})
}

func postAddPermission(t *testing.T, server *Server, rule string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/api/permissions/add?rule="+url.QueryEscape(rule), nil))
	return response
}

// Fixture (writePermissionSettings): allow ls+cat, ask rm, disabled-allow jq.
func TestAddPermission(t *testing.T) {
	t.Run("new-rule-appended", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := postAddPermission(t, server, "Bash(rg *)")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Empty(t, response.Header().Get("HX-Trigger"))
		assert.Contains(t, response.Body.String(), ">allowed</span>")

		mainPerms, _ := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(ls *)", "Bash(cat *)", "Bash(rg *)"}, mainPerms.Allow)
	})

	t.Run("already-allowed-noop", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := postAddPermission(t, server, "Bash(ls *)")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Empty(t, response.Header().Get("HX-Trigger"))
		assert.Contains(t, response.Body.String(), ">allowed</span>")

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(ls *)", "Bash(cat *)"}, mainPerms.Allow)
		assert.Equal(t, []string{"Bash(jq *)"}, disabledPerms.Allow)
	})

	t.Run("already-ask-noop", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := postAddPermission(t, server, "Bash(rm *)")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Empty(t, response.Header().Get("HX-Trigger"))
		assert.Contains(t, response.Body.String(), ">ask</span>")

		mainPerms, _ := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(rm *)"}, mainPerms.Ask)
	})

	t.Run("parked-allow-moved-back", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := postAddPermission(t, server, "Bash(jq *)")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Empty(t, response.Header().Get("HX-Trigger"))

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(ls *)", "Bash(cat *)", "Bash(jq *)"}, mainPerms.Allow)
		assert.Empty(t, disabledPerms.Allow)
	})

	t.Run("parked-ask-moved-to-allow", func(t *testing.T) {
		settingsPath := filepath.Join(t.TempDir(), "settings.json")
		require.NoError(t, config.Save(settingsPath, config.NewSettings(), nil))
		disabled := config.NewSettings()
		require.NoError(t, disabled.SetPermissions(config.Permissions{Ask: []string{"Bash(rg *)"}}))
		require.NoError(t, config.Save(config.DisabledPath(settingsPath), disabled, nil))
		server := newTestServer(t, &Options{SettingsPath: settingsPath})

		response := postAddPermission(t, server, "Bash(rg *)")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(rg *)"}, mainPerms.Allow)
		assert.Empty(t, disabledPerms.Ask)
	})

	t.Run("empty-rule-400", func(t *testing.T) {
		_, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/permissions/add", nil))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("non-rule-400", func(t *testing.T) {
		_, server := writePermissionSettings(t)
		response := postAddPermission(t, server, "rm -rf /")
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})
}

func TestPermissionsSectionOverriddenBadge(t *testing.T) {
	perms := `{"permissions": {"allow": ["Bash(ls *)"]}}`

	drifted := newSectionServer(t, perms, `{"permissions": {"allow": []}}`)
	assertSectionBadge(t, drifted, "/api/permissions", "permissions", true)

	clean := newSectionServer(t, perms, perms)
	assertSectionBadge(t, clean, "/api/permissions", "permissions", false)

	// defaultMode is the Claude tab's key: a bypass mode alone is no rule override.
	bypass := newSectionServer(t, `{"permissions": {"allow": ["Bash(ls *)"], "defaultMode": "bypassPermissions"}}`, perms)
	assertSectionBadge(t, bypass, "/api/permissions", "permissions", false)
}

// Rows render alphabetically across main+disabled and stay in place after a
// toggle (D7); the fixture's file order is ls, cat, (disabled) jq.
func TestPermissionsSorted(t *testing.T) {
	assertSorted := func(t *testing.T, body string) {
		t.Helper()
		catAt := strings.Index(body, "Bash(cat *)")
		jqAt := strings.Index(body, "Bash(jq *)")
		lsAt := strings.Index(body, "Bash(ls *)")
		require.Greater(t, catAt, 0)
		require.Greater(t, jqAt, 0)
		require.Greater(t, lsAt, 0)
		assert.Less(t, catAt, jqAt)
		assert.Less(t, jqAt, lsAt)
	}

	_, server := writePermissionSettings(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/permissions", nil))
	require.Equal(t, 200, response.Code)
	assertSorted(t, response.Body.String())

	// Toggling ls (main index 0) moves it between files but not in the list.
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/api/permissions/allow/0/toggle", nil))
	require.Equal(t, 200, response.Code, response.Body.String())
	assertSorted(t, response.Body.String())
}

func TestPermissionMutationAnswersWithFragment(t *testing.T) {
	_, server := writePermissionSettings(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/api/permissions/allow/0/toggle", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Empty(t, response.Header().Get("HX-Trigger"))

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/permissions", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Empty(t, response.Header().Get("HX-Trigger"))
}

func TestPermissionRows(t *testing.T) {
	t.Run("marks-shared-local-and-repo-only", func(t *testing.T) {
		main := []string{"Bash(ls *)", "Bash(cat *)"} // ls shared, cat local
		disabled := []string{"Bash(jq *)"}            // jq local (parked)
		frag := []string{"Bash(ls *)", "Bash(rm *)"}  // rm repo-only
		rows := permissionRows(main, disabled, frag, listAllow, listDisabledAllow, true)

		bySource := map[string]permissionRow{}
		for _, row := range rows {
			bySource[row.Value] = row
		}
		assert.Equal(t, sourceShared, bySource["Bash(ls *)"].Source)
		assert.Equal(t, sourceLocal, bySource["Bash(cat *)"].Source)
		assert.Equal(t, sourceLocal, bySource["Bash(jq *)"].Source)
		require.Contains(t, bySource, "Bash(rm *)")
		repoOnly := bySource["Bash(rm *)"]
		assert.Equal(t, sourceRepoOnly, repoOnly.Source)
		assert.False(t, repoOnly.Enabled)
		assert.Empty(t, repoOnly.List)
	})

	t.Run("unknown-fragment-skips-marking", func(t *testing.T) {
		rows := permissionRows([]string{"Bash(ls *)"}, nil, nil, listAllow, listDisabledAllow, false)
		require.Len(t, rows, 1)
		assert.Empty(t, rows[0].Source)
	})
}

func TestPermissionsLocalAndRepoOnlyMarking(t *testing.T) {
	live := `{"permissions": {"allow": ["Bash(ls *)", "Bash(cat *)"]}}`    // ls shared, cat local
	fragment := `{"permissions": {"allow": ["Bash(ls *)", "Bash(rm *)"]}}` // rm repo-only
	server := newSectionServer(t, live, fragment)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/permissions", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	body := response.Body.String()

	assert.Contains(t, body, "Bash(rm *)")        // repo-only rule surfaced as a row
	assert.Contains(t, body, ">local</span>")     // cat marked local (orange)
	assert.Contains(t, body, ">Repo Only</span>") // rm marked repo-only (grey)
}

func TestPermissionsGroupedByHead(t *testing.T) {
	live := `{"permissions": {"allow": ["mcp__peek-mcp__session_get", "Edit", "Bash(ls *)", "Bash(cat *)"], "deny": ["TaskStop"]}}`
	server := newSectionServer(t, live, live)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/permissions", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	body := response.Body.String()

	bashAt := strings.Index(body, `id="perms-allow-Bash"`)
	editAt := strings.Index(body, `id="perms-allow-Edit"`)
	mcpAt := strings.Index(body, `id="perms-allow-mcp"`)
	require.Positive(t, bashAt)
	require.Greater(t, editAt, bashAt)
	require.Greater(t, mcpAt, editAt)

	catAt := strings.Index(body, "Bash(cat *)")
	lsAt := strings.Index(body, "Bash(ls *)")
	assert.Greater(t, catAt, bashAt)
	assert.Less(t, catAt, lsAt)
	assert.Less(t, lsAt, editAt)
	assert.Greater(t, strings.Index(body, "mcp__peek-mcp__session_get"), mcpAt)

	assert.Contains(t, body, `id="perms-deny-TaskStop"`)
	assert.Contains(t, body, `hx-post="/api/permissions/deny/0/toggle"`)
	assert.Contains(t, body, `hx-post="/api/permissions/deny/items"`)
}

func TestRuleHead(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		value string
	}

	tests := make([]*testCase, 0)

	// tool-with-spec
	tests = append(tests, &testCase{
		_id:       "tool-with-spec",
		_expected: "Bash",

		value: "Bash(git log*)",
	})

	// mcp-with-tool
	tests = append(tests, &testCase{
		_id:       "mcp-with-tool",
		_expected: "mcp",

		value: "mcp__peek-mcp__session_get",
	})

	// mcp-server-only
	tests = append(tests, &testCase{
		_id:       "mcp-server-only",
		_expected: "mcp",

		value: "mcp__serena",
	})

	// bare-tool
	tests = append(tests, &testCase{
		_id:       "bare-tool",
		_expected: "Edit",

		value: "Edit",
	})

	// webfetch-domain
	tests = append(tests, &testCase{
		_id:       "webfetch-domain",
		_expected: "WebFetch",

		value: "WebFetch(domain:docs.rs)",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, ruleHead(test.value))
		})
	}
}

// Fixture (writePermissionSettings): allow ls+cat, ask rm, deny TaskStop, defaultMode acceptEdits, disabled-allow jq.
func TestRemovePermission(t *testing.T) {
	t.Run("allow-row-removed-keeps-defaultMode", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		permissionItemsHandler(server).ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/permissions/allow/0", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Empty(t, response.Header().Get("HX-Trigger"))

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(cat *)"}, mainPerms.Allow)
		assert.Equal(t, []string{"TaskStop"}, mainPerms.Deny)
		assert.Equal(t, []string{"Bash(jq *)"}, disabledPerms.Allow)
		assert.Equal(t, `"acceptEdits"`, loadDefaultMode(t, settingsPath))
	})

	t.Run("disabled-row-removed-from-sidecar", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		permissionItemsHandler(server).ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/permissions/disabledAllow/0", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())

		mainPerms, disabledPerms := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(ls *)", "Bash(cat *)"}, mainPerms.Allow)
		assert.Empty(t, disabledPerms.Allow)
		assert.Equal(t, `"acceptEdits"`, loadDefaultMode(t, settingsPath))
	})

	t.Run("index-out-of-range-404", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		permissionItemsHandler(server).ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/permissions/allow/99", nil))
		assert.Equal(t, http.StatusNotFound, response.Code)

		mainPerms, _ := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(ls *)", "Bash(cat *)"}, mainPerms.Allow)
	})

	t.Run("unknown-list-400", func(t *testing.T) {
		_, server := writePermissionSettings(t)
		response := httptest.NewRecorder()
		permissionItemsHandler(server).ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/permissions/bogus/0", nil))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})
}

func postPermissionItem(t *testing.T, server *Server, list, value string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	form := url.Values{"value": {value}}
	permissionItemsHandler(server).ServeHTTP(response, formPost("/api/permissions/"+list+"/items", form))
	return response
}

// Fixture (writePermissionSettings): allow ls+cat, ask rm, deny TaskStop, defaultMode acceptEdits, disabled-allow jq.
func TestAddPermissionItem(t *testing.T) {
	t.Run("appends-to-deny", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := postPermissionItem(t, server, listDeny, " Bash(rm -rf *) ")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Empty(t, response.Header().Get("HX-Trigger"))

		mainPerms, _ := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"TaskStop", "Bash(rm -rf *)"}, mainPerms.Deny)
		assert.Equal(t, []string{"Bash(ls *)", "Bash(cat *)"}, mainPerms.Allow)
		assert.Equal(t, `"acceptEdits"`, loadDefaultMode(t, settingsPath))
	})

	t.Run("creates-missing-list", func(t *testing.T) {
		settingsPath := filepath.Join(t.TempDir(), "settings.json")
		require.NoError(t, os.WriteFile(settingsPath, []byte(`{"permissions": {"defaultMode": "acceptEdits"}}`), 0644))
		server := newTestServer(t, &Options{SettingsPath: settingsPath})

		response := postPermissionItem(t, server, listAllow, "Edit")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())

		mainPerms, _ := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Edit"}, mainPerms.Allow)
		assert.Equal(t, `"acceptEdits"`, loadDefaultMode(t, settingsPath))
	})

	t.Run("duplicate-400", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := postPermissionItem(t, server, listAllow, "Bash(ls *)")
		assert.Equal(t, http.StatusBadRequest, response.Code)

		mainPerms, _ := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(ls *)", "Bash(cat *)"}, mainPerms.Allow)
	})

	t.Run("empty-400", func(t *testing.T) {
		_, server := writePermissionSettings(t)
		response := postPermissionItem(t, server, listAllow, "   ")
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("disabled-list-name-400", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := postPermissionItem(t, server, listDisabledAllow, "Bash(rg *)")
		assert.Equal(t, http.StatusBadRequest, response.Code)

		_, disabledPerms := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(jq *)"}, disabledPerms.Allow)
	})

	t.Run("agent-rule-accepted", func(t *testing.T) {
		settingsPath, server := writePermissionSettings(t)
		response := postPermissionItem(t, server, listAllow, "Agent(*)")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())

		mainPerms, _ := loadPermissions(t, settingsPath)
		assert.Equal(t, []string{"Bash(ls *)", "Bash(cat *)", "Agent(*)"}, mainPerms.Allow)
	})
}
