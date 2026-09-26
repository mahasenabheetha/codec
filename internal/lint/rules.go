package lint

// rules is the catalogue shown in settings. Messages and fixes are
// written per finding; Why is the general reason the rule exists.
var rules = []Rule{
	// --- style ---
	{ID: "duplicate-key", Group: "style", Title: "Duplicate keys", Default: "error",
		Why: "Most tools silently keep only the last value, so the first one is ignored without a word."},
	{ID: "truthy", Group: "style", Title: "yes/no/on/off values", Default: "warning",
		Why: "YAML 1.1 readers (Kubernetes' sigs.k8s.io/yaml, Ansible, PyYAML) turn yes/no/on/off into booleans; YAML 1.2 readers keep them as text. The same file means different things to different tools."},
	{ID: "octal", Group: "style", Title: "Octal-looking numbers", Default: "warning",
		Why: "A number with a leading zero is octal in YAML 1.1 (0755 = 493) but decimal in YAML 1.2 (755), so tools disagree on its value."},
	{ID: "indent", Group: "style", Title: "Indentation consistency", Default: "info",
		Why: "Mixed indentation widths make nesting hard to see, and a line that is one space off silently moves to another parent."},
	{ID: "empty-value", Group: "style", Title: "Empty values", Default: "info",
		Why: "A key with nothing after it is null. Tools either drop it or fail where they expect a map or list; usually a value was forgotten or mis-indented."},
	{ID: "trailing-spaces", Group: "style", Title: "Trailing spaces", Default: "info",
		Why: "Invisible trailing whitespace creates noisy diffs, and inside block scalars (|) it becomes part of the value."},
	{ID: "document-start", Group: "style", Title: "Document start (---)", Default: "off",
		Why: "Some teams require every file to start with ---, so documents concatenate safely."},
	{ID: "line-length", Group: "style", Title: "Line length", Default: "off",
		Why: "Very long lines are hard to read and review. Long words such as URLs are allowed."},

	// --- kubernetes ---
	{ID: "image-tag", Group: "kubernetes", Title: "Image without a pinned tag", Default: "warning",
		Why: ":latest (or no tag) moves: a restart or a new node can pull a different image than the one you tested, and a rollback can't return to the old one."},
	{ID: "resources", Group: "kubernetes", Title: "Missing requests/limits", Default: "warning",
		Why: "Without requests the scheduler can't place the pod sensibly and it is evicted first under pressure; without a memory limit one container can starve the whole node."},
	{ID: "probes", Group: "kubernetes", Title: "Missing readiness probe", Default: "warning",
		Why: "Without a readiness probe, traffic reaches the pod as soon as the container starts, before the app can serve it, and a rolling update can't tell a broken release from a good one."},
	{ID: "privileged", Group: "kubernetes", Title: "Privileged containers", Default: "warning",
		Why: "A privileged container has every capability and all host devices: escaping to the node is trivial."},
	{ID: "host-path", Group: "kubernetes", Title: "hostPath volumes", Default: "warning",
		Why: "hostPath exposes the node's filesystem to the pod, ties the pod to one node, and is a common container-escape route."},
	{ID: "host-network", Group: "kubernetes", Title: "hostNetwork", Default: "warning",
		Why: "The pod shares the node's network: it sees all host traffic and can bind host ports, bypassing NetworkPolicies."},
	{ID: "run-as-non-root", Group: "kubernetes", Title: "Missing runAsNonRoot", Default: "warning",
		Why: "Containers run as root unless told otherwise. A compromised root process has far more room to break out, and the restricted Pod Security level rejects the pod."},

	// --- deprecation ---
	{ID: "api-removed", Group: "deprecation", Title: "Removed APIs", Default: "error",
		Why: "The API server of the target Kubernetes version doesn't serve this API: kubectl apply and Helm fail with \"no matches for kind\"."},
	{ID: "api-deprecated", Group: "deprecation", Title: "Deprecated APIs", Default: "warning",
		Why: "Deprecated APIs are removed in a later release; moving now avoids a failed deploy after a cluster upgrade."},

	// --- schema ---
	{ID: "schema", Group: "schema", Title: "Schema validation", Default: "error",
		Why: "The file doesn't match the published schema for its type (Kubernetes, CRD, GitHub Actions, GitLab CI, Azure Pipelines, Compose), so the tool that reads it will reject it or ignore parts of it."},
}
