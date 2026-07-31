package secretscan

// Data only — reviewing "what do we detect" is a diff of this file, never a
// code read. Patterns curated from public key-format documentation.

const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
)

const detectorDenyFile = "deny-file"

type patternSpec struct {
	confidence string
	id         string
	regex      string
}

var patternSpecs = []patternSpec{
	{ConfidenceHigh, "aws-access-key-id", `\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`},
	{ConfidenceHigh, "credentialed-url", `\b(?:postgres|postgresql|mysql|redis|rediss|amqp|amqps|mongodb|mongodb\+srv)://[^:/\s'"@]+:[^@\s'"]{4,}@`},
	{ConfidenceHigh, "django-secret-key", `(?m)^[ \t]*SECRET_KEY\s*=\s*["'][^"']{16,}["']`},
	{ConfidenceHigh, "github-fine-grained-token", `\bgithub_pat_[A-Za-z0-9_]{22,255}\b`},
	{ConfidenceHigh, "github-token", `\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36,255}\b`},
	{ConfidenceHigh, "go-mysql-dsn", `\b[A-Za-z0-9_]+:[^@\s'"]{4,}@tcp\(`},
	{ConfidenceHigh, "google-api-key", `\bAIza[0-9A-Za-z_\-]{35}\b`},
	{ConfidenceHigh, "jwt", `\beyJ[A-Za-z0-9_\-]{14,}\.[A-Za-z0-9_\-]{14,}\.[A-Za-z0-9_\-]{10,}\b`},
	{ConfidenceHigh, "pem-private-key", `-----BEGIN [A-Z ]*PRIVATE KEY-----(?:[\sA-Za-z0-9+/=\\]*?-----END [A-Z ]*PRIVATE KEY-----)?`},
	{ConfidenceHigh, "slack-token", `\bxox[abprs]-[A-Za-z0-9\-]{10,250}\b`},
	{ConfidenceHigh, "stripe-live-key", `\b[rs]k_live_[0-9a-zA-Z]{20,247}\b`},
	{ConfidenceMedium, "env-assignment", `(?im)^\s*(?:export\s+)?[A-Za-z0-9_]*(?:KEY|TOKEN|SECRET|PASSW)[A-Za-z0-9_]*\s*=\s*[^\s#'"$][^\s#'"]{5,}[ \t]*$`},
	{ConfidenceMedium, "generic-assignment", `(?i)\b(?:password|passwd|secret|token|api[_-]?key)\s*[:=]\s*["'][^"'\s]{8,}["']`},
	{ConfidenceMedium, "go-setenv-credential", `os\.Setenv\(\s*"[A-Z0-9_]*(?:KEY|TOKEN|SECRET|PASSWORD|PASS|DSN)[A-Z0-9_]*"\s*,\s*"[^"]{4,}"\)`},
}

var denyFileGlobs = []string{
	".env*",
	".netrc",
	".pgpass",
	"*.jks",
	"*.keystore",
	"*.p12",
	"*.pem",
	"*.pfx",
	"*.ppk",
	"credentials.json",
	"id_dsa*",
	"id_ecdsa*",
	"id_ed25519*",
	"id_rsa*",
	"kubeconfig*",
	"local_settings.py",
	"service-account*.json",
	"settings_local.py",
}

var skipDirNames = []string{
	".git",
	".idea",
	".mypy_cache",
	".pytest_cache",
	".ruff_cache",
	".terraform",
	".tox",
	".venv",
	"__pycache__",
	"dist",
	"node_modules",
	"vendor",
	"venv",
}

var skipDirPaths = []string{
	".claude/worktrees",
}

var skipFileGlobs = []string{
	".git",
	".secretscan-baseline",
	"*.map",
	"*.min.css",
	"*.min.js",
	"*.svg",
	"*_test.go",
	"Cargo.lock",
	"Pipfile.lock",
	"composer.lock",
	"go.sum",
	"package-lock.json",
	"pnpm-lock.yaml",
	"poetry.lock",
	"uv.lock",
	"yarn.lock",
}

// mediaContextKeywords suppress base64-entropy findings near data-URI markers —
// embedded media (PNG/SVG data URIs in built assets) is maximum-entropy but
// structurally identifiable by its prefix.
var mediaContextKeywords = []string{
	";base64,",
	"data:image",
}

// hashContextKeywords suppress hex-entropy findings on lines that name a
// digest — content hashes are maximum-entropy and indistinguishable from keys
// by entropy alone.
var hashContextKeywords = []string{
	"blake2",
	"checksum",
	"digest",
	"etag",
	"integrity",
	"md5",
	"sha-",
	"sha1",
	"sha256",
	"sha384",
	"sha512",
}
