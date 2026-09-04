<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->

# PYTHON CODE STYLE RULES

**Files:** `*.py`

## BASELINE

All code MUST follow the rules in this document.

Where this document is silent, prefer clarity over cleverness.

---

## TAGS

Every rule carries exactly one tag:

* `[lint]` — CI proves it; the proving tool is named in the rule's `Lint:` bullet. **Do not check
  these in review**; their absence from a review is not a gap. They are documented here for
  authors, and can be deleted from this guide once the team trusts the gate.
* `[review]` — no tool covers it. This is where review effort belongs.

A tag is a claim about what CI actually runs. If you add or remove a check, update the tag in the
same change — a rule tagged `[lint]` that CI does not really enforce is checked by **nobody**.
`make lint` runs `check-tags.sh` first, which proves every `[lint]` tag by feeding the tool a
deliberate violation and requiring the finding. Adding a tag means adding its probe there.

CI reports only findings on lines the branch changed, measured against the merge base with
`develop`. A green build means the code you touched conforms, **not** that the whole file does.

Import blocks are the one exception: `I001` is reported against the whole block, so adding an import
to a block whose ordering predates the gate surfaces it, and `make lint-autofix` sorts the block.
The Go repositories make the same trade with `goimports`.

CI also runs general-purpose checks this guide never documents — unused imports and variables,
invalid escape sequences, `type(x) == T` comparisons, mutable default arguments. Those findings are
the gate's business: do not re-derive them in review, and do not expect a `RULE-*` id for them.

**What the gate does not read:** `*/migrations/*` (South-generated, and RULE-PYTHON-QUOTE-001
exempts them by name) and 24 modules that still use Python 2 `print` statements, which the linter
cannot parse. `ruff.toml` lists them; that list is temporary and must shrink to nothing.

Run `make lint` before pushing. `make lint-autofix` fixes what is mechanical, and touches only the
lines your branch changed.

---

## NAMING

**RULE-PYTHON-NAME-001** `[review]` — Class names use PascalCase.

* Class names MUST use PascalCase

```python
class UserProfile(models.Model):
class FeatureMatrix(models.Model):
```

**RULE-PYTHON-NAME-002** `[review]` — Field, method, and variable names use snake_case.

* Field names, method names, and variable names MUST use snake_case

```python
screen_name = models.CharField(...)
def is_active(self):
```

**RULE-PYTHON-NAME-003** `[review]` — Constants use UPPER_SNAKE_CASE.

* Constants MUST use UPPER_SNAKE_CASE

```python
STATE_UNKNOWN = "unknown"
PAYMENTSOURCE_APPSTORE = "as"
```

**RULE-PYTHON-NAME-004** `[review]` — Static factory methods use PascalCase.

* Static factory methods MUST use PascalCase

```python
@staticmethod
def FromDict(data):

@staticmethod
def FromFeature(feature, plan_id):
```

**RULE-PYTHON-NAME-005** `[review]` — Private methods are prefixed with a single underscore.

* Private methods MUST be prefixed with a single underscore

```python
def _validate_signature(self):
def _prefetch_products(self):
```

**RULE-PYTHON-NAME-006** `[review]` — Admin helper display methods are prefixed with a single underscore.

* Admin helper display methods MUST be prefixed with a single underscore

```python
def _app_group(self, obj):
def _userprofile(self, obj):
```

---

## WHITESPACE

**RULE-PYTHON-WS-001** `[lint]` — Keyword arguments use spaces around `=`.

* Lint: yapf
* Keyword arguments MUST use spaces around `=`

```python
# Do:
models.CharField(max_length = 100, blank = True, default = "")
Product.objects.filter(id__in = product_ids)

# Do NOT:
models.CharField(max_length=100, blank=True, default="")
Product.objects.filter(id__in=product_ids)
```

* This applies to: model field declarations, function keyword arguments, queryset filters, and any other keyword argument usage

**RULE-PYTHON-WS-002** `[lint]` — Assignment statements have spaces around `=`.

* Lint: yapf
* Assignment statements MUST have spaces around `=`

```python
# Do:
self.name = ""
product_id = 123

# Do NOT:
self.name=""
product_id=123
```

**RULE-PYTHON-WS-003** `[lint]` — A trailing comma and a multi-line layout go together — wrapped constructs end with a comma, one-liners never carry one.

* Lint: yapf
* A trailing comma and a multi-line layout go together, in both directions:
  * A call, collection or import spanning MULTIPLE lines MUST end its last element with a comma
  * A construct on ONE line MUST NOT carry one

* The trailing comma is what tells the formatter to keep the wrapping. Without it the formatter
  joins the construct onto a single line, however long that line becomes

```python
# Do: wrapped, so it keeps the comma — the formatter leaves it alone
raise exceptions.PermissionDeniedException(
    "Login required for channel %s (%s)" % (self.channel_id, self.channel.name),
    user_message = "Bitte melde Dich mit Deinem TV.de Nutzerkonto an",
    user_title = "Anmeldung erforderlich",
)

# Do: one line, so no trailing comma
fields = ("name", "simultaneous_streams", "device_limit", "device_cooldown_days")

# Do NOT: wrapped without the comma — the formatter collapses this into one 339-character line
raise exceptions.PermissionDeniedException(
    "Login required for channel %s (%s)" % (self.channel_id, self.channel.name),
    user_message = "Bitte melde Dich mit Deinem TV.de Nutzerkonto an",
    user_title = "Anmeldung erforderlich"
)

# Do NOT: one line WITH a trailing comma — the formatter explodes this across six lines
fields = ("name", "simultaneous_streams", "device_limit", "device_cooldown_days",)
```

* EXCEPTION: a single-element tuple ALWAYS keeps its comma, on one line or many. The comma is what
  makes it a tuple — `("name",)` is a tuple, `("name")` is the string `"name"`

```python
ordering = ("name",)
```

* EXCEPTION: Python 2.7 rejects a trailing comma after `*args` / `**kwargs`, in both calls and
  definitions. Those never take one

```python
# SyntaxError under Python 2.7:
def handle(self, *args, **options,):
logging.warn(message, **extra,)
```

---

## QUOTING

**RULE-PYTHON-QUOTE-001** `[lint]` — String literals use double quotes.

* Lint: ruff Q000, Q003
* String literals MUST use double quotes

```python
# Do:
STATE_EXPIRED = "expired"
client_ip = models.TextField(blank = True, default = "")

# Do NOT:
STATE_EXPIRED = 'expired'
client_ip = models.TextField(blank = True, default = '')
```

* Single quotes are allowed ONLY to avoid escaping an embedded double quote:

```python
message = 'He said "hello"'
```

* EXCEPTION: South-generated migrations use single quotes — do NOT reformat them.

---

## CLASS STRUCTURE

**RULE-PYTHON-CLASS-001** `[review]` — Elements within a class follow the canonical order shown below.

Elements within a class MUST be ordered as follows:

1. Manager declarations (`objects`, `cached`)
2. Constants (UPPER_SNAKE_CASE)
3. Model fields (for Django models)
4. Class-level variables
5. Inner classes (`class Meta`)
6. Static factory methods (PascalCase) — sorted alphabetically
7. `__init__`
8. Dunder methods (`__str__`, `__unicode__`, `__repr__`) — sorted alphabetically
9. Private methods (prefixed `_`) — sorted alphabetically
10. Public methods — sorted alphabetically
11. Properties — sorted alphabetically

**RULE-PYTHON-CLASS-002** `[review]` — Constants are grouped by their prefix, matching the field they describe.

* Constants MUST be grouped by their prefix (matching the field they describe)
* Within each group, constants MUST be sorted alphabetically
* The `_CHOICES` tuple MUST come last within its group

```python
STATE_CHARGEBACK = "chargeback"
STATE_FAILED = "failed"
STATE_PROCESSING = "processing"
STATE_READY = "ready"
STATE_UNKNOWN = "unknown"
STATE_CHOICES = (
    (STATE_CHARGEBACK, "Zurückgebucht"),
    (STATE_FAILED, "Fehlgeschlagen"),
    (STATE_PROCESSING, "In Bearbeitung"),
    (STATE_READY, "Bereit"),
    (STATE_UNKNOWN, "Unbekannt"),
)
```

**RULE-PYTHON-CLASS-003** `[lint]` — Classes are separated by exactly two blank lines.

* Lint: yapf
* Classes MUST be separated by exactly two blank lines

---

## MODEL FIELDS

**RULE-PYTHON-MODEL-001** `[review]` — Model fields are sorted alphabetically.

* Model fields MUST be sorted alphabetically

```python
class Purchase(models.Model):
    android_receipt = models.TextField(blank = True, default = "")
    app_group = models.ForeignKey("apps.AppGroup")
    cancelled_at = models.DateTimeField(null = True, blank = True)
    created_at = models.DateTimeField(auto_now_add = True)
```

**RULE-PYTHON-MODEL-002** `[review]` — `null = True` requires `blank = True`.

* If `null = True` is set, `blank = True` MUST also be set

```python
cancelled_at = models.DateTimeField(null = True, blank = True)
```

**RULE-PYTHON-MODEL-003** `[review]` — A set `default` requires `blank = True`, except on choice fields with a valid default.

* If `default` is set, `blank = True` MUST also be set (to allow blank in admin)

```python
ip = models.CharField(max_length = 50, default = "", blank = True)
```

* EXCEPTION: a field with `choices` and a valid non-empty `default` MUST NOT set `blank = True`. The default already satisfies the admin form; adding `blank = True` inserts an empty option that allows saving a value outside the defined choices.

```python
state = models.CharField(max_length = 50, choices = STATE_CHOICES, default = STATE_PROCESSING)
```

**RULE-PYTHON-MODEL-004** `[review]` — Field arguments that only repeat Django's built-in default are omitted.

* Field arguments that only repeat Django's built-in default MUST be omitted

```python
# Do:
enable_npvr = models.BooleanField(default = False)
max_sessions = models.IntegerField(default = 1)
external_id = models.CharField(max_length = 100, unique = True)

# Do NOT:
enable_npvr = models.BooleanField(null = False, blank = False, default = False)
max_sessions = models.IntegerField(null = False, default = 1)
external_id = models.CharField(max_length = 100, unique = True, db_index = False)
```

Common Django field defaults to omit unless you need to override them:
`null = False`, `blank = False`, `db_index = False`, `editable = True`, `primary_key = False`, `unique = False`

**RULE-PYTHON-MODEL-005** `[review]` — Datetime fields use either `auto_now = True` or `auto_now_add = True` — never both.

* Datetime fields MUST use either `auto_now = True` or `auto_now_add = True` — never both

```python
created_at = models.DateTimeField(auto_now_add = True)
updated_at = models.DateTimeField(auto_now = True)
```

**RULE-PYTHON-MODEL-006** `[review]` — Foreign keys use the entity name — never an appended `_id`.

* Foreign keys MUST use the entity name — do NOT append `_id` to the field name

```python
# Do:
plan = models.ForeignKey("Plan")
app_group = models.ForeignKey("apps.AppGroup")

# Do NOT:
plan_id = models.ForeignKey("Plan")
```

**RULE-PYTHON-MODEL-007** `[review]` — Manager declarations come before constants and fields.

* Manager declarations MUST come before constants and fields

```python
class Plan(models.Model):
    objects = models.Manager()
    cached = CacheByIdsManager()

    ID_FERNSEHENMITHERZ = "fernsehen_mit_herz"
    ...
```

---

## SORTING

**RULE-PYTHON-SORT-001** `[review]` — Model fields are sorted alphabetically (see RULE-PYTHON-MODEL-001).

* Model fields MUST be sorted alphabetically (see RULE-PYTHON-MODEL-001)

**RULE-PYTHON-SORT-002** `[review]` — Methods within each visibility group are sorted alphabetically.

* Methods within each visibility group (private, public) MUST be sorted alphabetically

**RULE-PYTHON-SORT-003** `[review]` — Static factory methods are sorted alphabetically.

* Static factory methods MUST be sorted alphabetically

**RULE-PYTHON-SORT-004** `[review]` — Classes in a file are sorted alphabetically; a base class still precedes its subclasses.

* Classes in a file MUST be sorted alphabetically
* A base class MUST still precede its subclasses (Python requires it). When this conflicts with alphabetical order
  within one file — i.e. a base class's name would sort after its own subclasses — split the module into a package: put
  subclassed base classes in `base.py` and leaf classes in `errors.py`, each alphabetically sorted, and re-export both
  from `__init__.py`. See `api/exceptions/` for the reference layout.

**RULE-PYTHON-SORT-005** `[review]` — `admin.site.register(...)` calls are sorted alphabetically at the bottom of `admin.py`.

* `admin.site.register(...)` calls MUST be sorted alphabetically at the bottom of `admin.py`

```python
admin.site.register(Ad, AdAdmin)
admin.site.register(AffiliateCampaign, AffiliateCampaignAdmin)
admin.site.register(Purchase, PurchaseAdmin)
```

---

## `__str__` AND `__unicode__`

**RULE-PYTHON-STR-001** `[review]` — `__str__` / `__unicode__` return a string in the canonical format shown below.

* `__str__` / `__unicode__` MUST return a string in the format:

```
<ClassName: field1 = value1, field2 = value2, ...>
```

* Fields MUST be listed alphabetically

```python
def __str__(self):
    return "<Purchase: id = %s, app_group_id = %s, created_at = %s, state = %s>" % (
        self.id, self.app_group_id, self.created_at, self.state
    )
```

* EXCEPTION: exception classes MAY return `repr(self.value)` (the raw message) from `__str__` / `__unicode__`, since
  `str(exc)` is expected to yield the message for logging. See `api/exceptions/`.

**RULE-PYTHON-STR-002** `[review]` — Use `__str__` for Python 3 compatible code.

* Use `__str__` for Python 3 compatible code
* Use `__unicode__` with `u""` prefix for Python 2 models (legacy)

---

## `as_dict` / `to_dict`

**RULE-PYTHON-DICT-001** `[review]` — Dict keys in `as_dict()` / `to_dict()` list `id` first, then the remaining keys sorted alphabetically.

* Dict keys in `as_dict()` / `to_dict()` MUST list `id` first, then the remaining keys sorted alphabetically

```python
def as_dict(self):
    return {
        "id": self.id,
        "expires_at": self.expires_at,
        "plan_id": self.plan_id,
        "state": self.state,
    }
```

---

## VALIDATION

**RULE-PYTHON-VALIDATE-001** `[review]` — Validation methods are named `validate(self)`.

* Validation methods MUST be named `validate(self)`

* Each validated field MUST have a comment above its block with the field name

```python
def validate(self):
    # id
    if not is_non_blank_string(self.id):
        raise ValueError("Invalid field id: Must be a non-empty string")

    # position
    if type(self.position) is not int:
        raise ValueError("Invalid field position: Must be an int: %s" % type(self.position))

    if self.position < 0:
        raise ValueError("Invalid field position: Must not be negative: %d" % self.position)
```

**RULE-PYTHON-VALIDATE-002** `[review]` — Error messages from `validate()` follow the canonical format shown below.

* Error messages from `validate()` MUST follow the format:

```
Invalid field <name>: <reason>
```

**RULE-PYTHON-VALIDATE-003** `[review]` — Type guard errors follow the canonical format shown below.

* Type guard errors MUST follow the format:

```
Parameter <name> must be an instance of <Type>: <value> (<type>)
```

```python
if not isinstance(other, Purchase):
    raise TypeError("Parameter other must be an instance of Purchase: %s (%s)" % (other, type(other)))
```

---

## ERROR HANDLING

**RULE-PYTHON-ERR-001** `[review]` — Errors are handled with early returns or early raises — never deep nesting after a check.

* Errors MUST be handled with early returns or early raises — do not deeply nest happy-path code after an error check

**RULE-PYTHON-ERR-002** `[review]` — Exception messages do not end with a period.

* Exception messages MUST NOT end with a period

**RULE-PYTHON-ERR-003** `[review]` — User-facing errors raise a `UserFacingException` subclass, which owns the status code, payload, and error code.

* User-facing errors (those carrying a message meant to be shown to the end user) MUST raise a subclass of
  `UserFacingException` from `api.exceptions`
* The subclass — NOT the exception handler — owns the HTTP status code via its `status_code` attribute; do NOT hard-code
  a status for these in `handle_exceptions`
* Pick the subclass whose status fits the situation; add a new `UserFacingException` subclass (in
  `api/exceptions/errors.py`) rather than overloading an unrelated one:

| Subclass                      | Status | Use                                                                                                                                     |
|-------------------------------|--------|-----------------------------------------------------------------------------------------------------------------------------------------|
| `PermissionDeniedException`   | 403    | User may not access the resource in their current state (anonymous request for a login-gated live channel)                              |
| `ConflictException`           | 409    | Request conflicts with the current account/resource state (active subscription blocks deletion, missing profile name blocks commenting) |
| `RegionRestrictedException`   | 451    | Content blocked for licensing / geo reasons (Unavailable For Legal Reasons)                                                             |
| `ServiceUnavailableException` | 503    | Service intentionally/temporarily unavailable (e.g. channel maintenance mode)                                                           |

* The user-facing payload (`user_title`, `user_message`, `user_action`, `user_action_url`) is carried by the exception
  and rendered by `UserFacingException.as_dict()`; do NOT assemble those keys by hand in views or the handler
* The client-facing `error_code` also belongs to the subclass, NOT to `handle_exceptions`: `UserFacingException`
  defaults it to 18, `ServiceUnavailableException` overrides it to 13 (the code clients recognize for that dialog).
  Give a new subclass its own `error_code` class attribute only when clients must tell it apart; never leave a
  user-facing error at `-1`

**RULE-PYTHON-ERR-004** `[review]` — Internal invariant violations raise `UnexpectedStateException` (500), never a user-facing exception.

* Internal invariant violations ("should never happen" guards) MUST raise `UnexpectedStateException` (→ 500), NOT a
  user-facing exception
* `handle_exceptions` logs these at error level; pass `extra = { "request": request }` at the raise site so the request
  is captured

---

## IMPORTS

**RULE-PYTHON-IMPORT-001** `[lint]` — Imports are grouped in the canonical order, separated by blank lines.

* Lint: ruff I001

Imports MUST be grouped in this order, separated by blank lines:

1. Standard library
2. Third-party packages
3. Local / project imports

```python
import datetime
import logging

import pytz
import ujson
from django.db import models

from api.exceptions import InvalidArgumentException
from iap.models import Plan, Purchase
```

**RULE-PYTHON-IMPORT-002** `[lint]` — Within each group, `import X` statements come before `from X import Y`.

* Lint: ruff I001
* Within each group, `import X` statements MUST come before `from X import Y` statements
* Within each subgroup, imports MUST be sorted alphabetically

---

## ADMIN

**RULE-PYTHON-ADMIN-001** `[review]` — Admin class names follow the pattern `<Model>Admin`.

* Admin class names MUST follow the pattern `<Model>Admin`

```python
class PurchaseAdmin(admin.ModelAdmin):
class PlanAdmin(admin.ModelAdmin):
```

**RULE-PYTHON-ADMIN-002** `[review]` — Changelists never issue per-row (N+1) queries for relations shown in `list_display`.

* Changelists MUST NOT issue per-row (N+1) queries for relations shown in `list_display`

* Single-valued FORWARD relations (`ForeignKey`, `OneToOneField`) MUST be preloaded with `list_select_related` (a SQL JOIN), naming each relation explicitly:

```python
class PurchaseAdmin(admin.ModelAdmin):
    list_display = ("id", "_app_group", "_product",)
    list_select_related = ("app_group", "product")
```

* NEVER rely on `list_select_related = True`, or on Django's auto-detection (a raw FK field name in `list_display`), to preload a NULLABLE FK. Both emit a bare `select_related()`, which skips nullable relations (`query_utils.select_related_descend`: `not restricted and field.null`), so the N+1 remains. Name nullable FKs explicitly in the tuple.

* Multi-valued relations (reverse `ForeignKey`, `ManyToManyField`) cannot be expressed via `list_select_related`; preload them in `get_queryset` with `prefetch_related` (nested paths allowed):

```python
def get_queryset(self, request):
    qs = super(PlanAdmin, self).get_queryset(request)
    qs = qs.prefetch_related("livetv_channels", "appgroupplan_set__appgroup")
    return qs
```

* A display method iterating a prefetched relation MUST use the cached `.all()` as-is. Calling `.order_by()`, `.only()`, or `.filter()` on the manager builds a new queryset that bypasses the prefetch cache and reintroduces the per-row query — sort or slice in Python instead:

```python
def _apps(self, config):
    apps = sorted(config.apps.all(), key = lambda app: app.slug)  # NOT config.apps.all().order_by("slug")
```

* `InlineModelAdmin` does NOT honor `list_select_related`; override its `get_queryset` and apply `select_related` / `prefetch_related` there

**RULE-PYTHON-ADMIN-003** `[review]` — Custom admin display methods are prefixed with `_` and registered with `short_description`.

* Custom display methods in admin MUST be prefixed with `_` and registered with `short_description`

```python
def _app_group(self, purchase):
    ...

_app_group.allow_tags = True
_app_group.short_description = "App Group"
```

**RULE-PYTHON-ADMIN-004** `[review]` — `admin.site.register(...)` calls sit at the bottom of the file, sorted alphabetically.

* `admin.site.register(...)` calls MUST appear at the bottom of the file, sorted alphabetically

---

## FORMS

**RULE-PYTHON-FORM-001** `[review]` — Request-validation form classes follow the canonical structure shown below.

* Form classes used for request validation (not Django `ModelForm`) MUST follow this structure:

1. `__init__` — store the request and any path/query parameters as instance variables
2. `validate` — fetch and validate all parameters; populate instance variables

```python
class GetProductsFormV1_1(object):
    def __init__(self, request, product_id = None):
        self.request = request
        self.product_id = product_id

    def validate(self):
        # Fetch parameters
        self.client_id = self.request.GET.get("app")

        # Validate parameters
        if not is_non_blank_string(self.client_id):
            raise InvalidArgumentException("Missing parameter app")
```

**RULE-PYTHON-FORM-002** `[review]` — Parameter validation errors use `InvalidArgumentException` with the canonical message format.

* Parameter validation errors MUST use `InvalidArgumentException` with a message in the format:

```
Missing parameter <name>
Invalid parameter <name>: <reason>
```

---

## MIGRATIONS

**RULE-PYTHON-MIGRATION-001** `[review]` — A model field added with a default gets an `# Enforcing defaults` SQL block in the migration's `forwards`.

* After adding a model field with a default value, add an `# Enforcing defaults` SQL block in the migration's `forwards` method:

```python
# Enforcing defaults
db.execute("ALTER TABLE iap_purchase ALTER COLUMN state SET DEFAULT 'unknown'")
```

* Both comment spellings are accepted: `# Enforcing defaults` and `# Enforce Defaults` — do NOT flag or rewrite one into the other

**RULE-PYTHON-MIGRATION-002** `[review]` — The trailing blank line before the `forwards` method is removed.

* Remove the trailing blank line before the `forwards` method (reformat with IDE after creation)

---

## MAPPING & COMPREHENSIONS

**RULE-PYTHON-MAP-001** `[review]` — Iterable-to-list/dict transformations use `map()` with a `lambda` — never comprehensions.

* Transforming an iterable into a list or dict MUST use `map()` with a `lambda` — NOT comprehensions or explicit iterator chains

* Comprehensions (`[x for ...]`, `{k: v for ...}`) and `.iterator()`-style chains read as Python-3-flavored and are avoided in this Python 2.7 codebase

```python
# Do:
appid_app_map = dict(map(lambda app: (app.id, app), apps))

# Do NOT:
appid_app_map = {app.id: app for app in apps}
```

**RULE-PYTHON-MAP-002** `[review]` — Mapping over a DB query selects only the needed columns via `.values(...)` / `.values_list(...)`.

* When mapping over a DB query, select only the needed columns with `.values(...)` / `.values_list(...)` so the lambda maps over lightweight rows instead of full model objects (lower memory, same style)

```python
# Do:
rows = User.objects.filter(userprofile_id__in = userprofile_ids).values("userprofile_id", "id")
userprofile_id_user_id_map = dict(map(lambda row: (str(row["userprofile_id"]), row["id"]), rows))
```

---

## API ENDPOINTS

---

## ENFORCEMENT

* These rules MUST be applied strictly
* Fix violations instead of discussing them
* Do NOT explain changes — only return corrected code
