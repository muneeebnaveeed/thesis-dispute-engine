package disputehttp

import (
	"fmt"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
)

type opClass string

const (
	public     opClass = "public"   // no credential
	internal   opClass = "internal" // service key, internal network only
	tenantWide opClass = "tenant"   // any authenticated caller; rows are scoped by RLS
	decided    opClass = "decided"  // the service asks the authorization engine
)

// operationClass says how each operation is authorized, keyed by the embedded spec's operation id (the handler
// method name); Mount refuses to start if the spec grows one it lacks.
var operationClass = map[string]opClass{
	"GetHealthz": public, "GetReadyz": public,
	"ListDisputes": tenantWide, "GetDispute": tenantWide, "GetAttachment": tenantWide, "GetNotice": tenantWide,
	"ListEmailTemplates": tenantWide, "GetTenant": tenantWide, "GetTenantLogo": tenantWide,
	"GetMyAvatar": tenantWide, "PutMyAvatar": tenantWide,
	"SuggestQuestionnaireAnswers": tenantWide, "SuggestSearchFilters": tenantWide, "SuggestDisputeReason": tenantWide,
	"CreateDispute": decided, "ApplyDisputeEvent": decided, "ComposeEmail": decided, "UploadAttachment": decided,
	"ResendNotice": decided, "ListTenantTemplates": decided, "PutTenantTemplate": decided,
	"DeleteTenantTemplate": decided, "PutTenantLogo": decided,
	"ListTenantKeys": decided, "CreateTenantKey": decided, "RevokeTenantKey": decided,
	"PutSession": internal, "GetSession": internal, "DeleteSession": internal, "DeleteSessions": internal, "ListTenants": internal,
}

func checkCoverage(spec *openapi3.T) error {
	var missing []string
	for _, item := range spec.Paths.Map() {
		for _, op := range item.Operations() {
			if _, ok := operationClass[op.OperationID]; !ok {
				missing = append(missing, op.OperationID)
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("disputehttp: operations without an authorization class: %v", missing)
	}
	return nil
}
