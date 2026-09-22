import { useQuery } from '@tanstack/react-query'
import { Link, useRouter, useRouterState } from '@tanstack/react-router'
import {
  EnvelopeSimpleIcon,
  HouseIcon,
  KeyIcon,
  MagnifyingGlassIcon,
  SignOutIcon,
  UploadSimpleIcon,
} from '@phosphor-icons/react'
import { useRef, type ChangeEvent } from 'react'

import { describe } from '#/api/failure'
import { Avatar, AvatarFallback, AvatarImage } from '#/components/shadcn/avatar'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '#/components/shadcn/sidebar'
import { brandingQuery, myAvatarQuery } from '#/queries/identity'
import { useServerMutation } from '#/queries/use-server-mutation'
import { cn } from '#/lib/utils'
import { putMyAvatar, putTenantLogo } from '#/server/functions/identity'
import { logout } from '#/server/functions/session'

type Viewer = {
  tenantSlug: string
  name: string
  firstName: string | null
  lastName: string | null
  roles: string[]
}

// what the API stores; the picker filters before the refusal travels
const PICKABLE_IMAGE_TYPES = 'image/png,image/jpeg,image/webp'

const upload = (file: File) => {
  const form = new FormData()
  form.append('file', file, file.name)
  return form
}

const initialsOf = (...parts: (string | null)[]) =>
  parts
    .flatMap((part) => (part ?? '').split(/\s+/))
    .filter(Boolean)
    .slice(0, 2)
    .map((word) => word.charAt(0).toUpperCase())
    .join('') || '?'

const PickableImage = ({
  src,
  initials,
  label,
  square,
  onPick,
}: {
  src: string | null
  initials: string
  label: string
  square?: boolean | undefined
  onPick?: ((file: File) => void) | undefined
}) => {
  const picker = useRef<HTMLInputElement>(null)
  const picture = (
    <Avatar className={cn(square && 'rounded-md after:rounded-md')}>
      {src && <AvatarImage src={src} alt="" className={cn(square && 'rounded-md')} />}
      <AvatarFallback className={cn('text-xs font-medium', square && 'rounded-md')}>
        {initials}
      </AvatarFallback>
    </Avatar>
  )
  if (!onPick) return picture
  const pick = (event: ChangeEvent<HTMLInputElement>) => {
    const [file] = event.target.files ?? []
    if (file) onPick(file)
    event.target.value = ''
  }
  return (
    <>
      <button
        type="button"
        aria-label={label}
        onClick={() => picker.current?.click()}
        className={cn(
          'group/pick relative cursor-pointer ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
          square ? 'rounded-md' : 'rounded-full',
        )}
      >
        {picture}
        <span
          aria-hidden
          className={cn(
            'absolute inset-0 flex items-center justify-center bg-black/55 text-white opacity-0 transition-opacity group-hover/pick:opacity-100 group-focus-visible/pick:opacity-100',
            square ? 'rounded-md' : 'rounded-full',
          )}
        >
          <UploadSimpleIcon className="size-4" weight="bold" />
        </span>
      </button>
      <input
        ref={picker}
        type="file"
        accept={PICKABLE_IMAGE_TYPES}
        className="sr-only"
        aria-hidden
        tabIndex={-1}
        onChange={pick}
      />
    </>
  )
}

export const AppSidebar = ({ viewer }: { viewer: Viewer }) => {
  const router = useRouter()
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const tenant = viewer.tenantSlug
  const isAdmin = viewer.roles.includes('tenant-admin')
  const { data: loadedBranding } = useQuery(brandingQuery())
  const { data: loadedAvatar } = useQuery(myAvatarQuery())
  const branding = loadedBranding?.value ?? null
  const logoUpload = useServerMutation((file: File) => putTenantLogo({ data: upload(file) }), {
    invalidates: () => [['branding']],
  })
  const avatarUpload = useServerMutation((file: File) => putMyAvatar({ data: upload(file) }), {
    invalidates: () => [['my-avatar']],
  })
  const refusal = logoUpload.failure ?? avatarUpload.failure
  const signOut = async () => {
    const { url: afterLogoutUrl } = await logout()
    if (afterLogoutUrl.startsWith('/')) await router.navigate({ to: afterLogoutUrl })
    else window.location.assign(afterLogoutUrl)
  }

  return (
    <Sidebar>
      <SidebarHeader className="flex-row items-center gap-2 border-b border-sidebar-border bg-[image:var(--panel-heading)] px-2 py-1.5">
        <PickableImage
          square
          src={branding?.logoDataUrl ?? null}
          initials={initialsOf(branding?.name ?? tenant)}
          label="Replace the organisation logo"
          onPick={isAdmin ? (file) => logoUpload.mutate(file) : undefined}
        />
        <span className="truncate text-[11px] font-bold capitalize">{branding?.name ?? tenant}</span>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup className="p-0">
          <SidebarGroupContent>
            <SidebarMenu className="gap-0">
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
      <SidebarFooter className="border-t border-sidebar-border bg-[image:var(--toolbar)] p-1">
        {isAdmin && (
          <SidebarMenu className="gap-0">
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
        )}
        {refusal && (
          <p role="alert" className="px-2 text-xs text-amber-700">
            {describe(refusal).title}
          </p>
        )}
        <div className="flex items-center gap-2 px-1 py-0.5 group-data-[collapsible=icon]:px-0">
          <PickableImage
            src={loadedAvatar?.value?.dataUrl ?? null}
            initials={initialsOf(viewer.firstName, viewer.lastName, viewer.name)}
            label="Replace your picture"
            onPick={(file) => avatarUpload.mutate(file)}
          />
          <span className="min-w-0 flex-1 truncate text-[11px] capitalize group-data-[collapsible=icon]:hidden">
            {viewer.name}
          </span>
          <button
            type="button"
            aria-label="Sign out"
            title="Sign out"
            onClick={() => void signOut()}
            className="cursor-pointer rounded-md p-2 text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground group-data-[collapsible=icon]:hidden"
          >
            <SignOutIcon />
          </button>
        </div>
      </SidebarFooter>
    </Sidebar>
  )
}
