import { Link, useRouter, useRouterState } from '@tanstack/react-router'
import {
  EnvelopeSimpleIcon,
  HouseIcon,
  KeyIcon,
  MagnifyingGlassIcon,
  SignOutIcon,
} from '@phosphor-icons/react'

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '#/components/shadcn/sidebar'
import { logout } from '#/server/functions/session'

type Viewer = { tenantSlug: string; name: string; roles: string[] }

export const AppSidebar = ({ viewer }: { viewer: Viewer }) => {
  const router = useRouter()
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const tenant = viewer.tenantSlug
  const isAdmin = viewer.roles.includes('tenant-admin')
  const signOut = async () => {
    const { url: afterLogoutUrl } = await logout()
    if (afterLogoutUrl.startsWith('/')) await router.navigate({ to: afterLogoutUrl })
    else window.location.assign(afterLogoutUrl)
  }

  return (
    <Sidebar>
      <SidebarHeader className="px-4 py-3">
        <span className="text-sm font-medium tracking-wide text-muted-foreground uppercase">
          Dispute Engine
        </span>
        <span className="text-xs text-muted-foreground">{tenant}</span>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              <SidebarMenuItem>
                <SidebarMenuButton asChild isActive={pathname === `/${tenant}`} tooltip="Disputes">
                  <Link to="/$tenant" params={{ tenant }}>
                    <HouseIcon />
                    <span>Disputes</span>
                  </Link>
                </SidebarMenuButton>
              </SidebarMenuItem>
              <SidebarMenuItem>
                <SidebarMenuButton asChild isActive={pathname === `/${tenant}/search`} tooltip="Search">
                  <Link to="/$tenant/search" params={{ tenant }}>
                    <MagnifyingGlassIcon />
                    <span>Search</span>
                  </Link>
                </SidebarMenuButton>
              </SidebarMenuItem>
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter>
        {isAdmin && (
          <SidebarGroup className="p-0">
            <SidebarGroupLabel>Admin panel</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                <SidebarMenuItem>
                  <SidebarMenuButton asChild isActive={pathname === `/${tenant}/keys`} tooltip="Tenant keys">
                    <Link to="/$tenant/keys" params={{ tenant }}>
                      <KeyIcon />
                      <span>Tenant keys</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
                <SidebarMenuItem>
                  <SidebarMenuButton
                    asChild
                    isActive={pathname === `/${tenant}/templates`}
                    tooltip="Email templates"
                  >
                    <Link to="/$tenant/templates" params={{ tenant }}>
                      <EnvelopeSimpleIcon />
                      <span>Email templates</span>
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        )}
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton onClick={() => void signOut()} tooltip={`Sign ${viewer.name} out`}>
              <SignOutIcon />
              <span className="truncate">Sign out, {viewer.name}</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </Sidebar>
  )
}
