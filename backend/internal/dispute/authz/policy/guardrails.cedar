// Separation of duties: whoever opened the investigation never issues its final credit.
@id("sod-final-credit")
forbid (principal is Analyst, action == Action::"ISSUE_FINAL_CREDIT", resource is Dispute)
when { resource.investigationOpenedBy == principal.id };

// Only a lead of the dispute's team may move it to another team.
@id("reassign-leads-only")
forbid (principal, action == Action::"reassign", resource is Dispute)
unless { principal is Analyst && principal.leadOf.contains(resource.team) };

// Tenant administration is the realm role, never a grant.
@id("tenant-admin")
permit (principal is Analyst,
        action in [Action::"manage-keys", Action::"manage-templates", Action::"manage-branding"],
        resource is Tenant)
when { principal.tenantAdmin };

// Tenant keys keep today's reach until profiles arrive: any dispute action, and opening disputes.
@id("keys-disputes")
permit (principal is TenantKey, action, resource is Dispute);

@id("keys-create")
permit (principal is TenantKey, action == Action::"create", resource is Team);

// Composing, attaching, resending and reassigning are for people.
@id("keys-not-people")
forbid (principal is TenantKey,
        action in [Action::"compose-email", Action::"upload-attachment", Action::"resend-notice", Action::"reassign"],
        resource);
