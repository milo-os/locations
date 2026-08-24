import { useState, type ReactNode } from "react";
import { Link, useLocation, useNavigation } from "@remix-run/react";
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarTrigger,
} from "@datum-cloud/datum-ui/sidebar";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@datum-cloud/datum-ui/breadcrumb";
import { ArrowLeftIcon, BoxIcon, LayoutDashboardIcon } from "lucide-react";

interface AppLayoutProps {
  children: ReactNode;
}

interface Crumb {
  label: string;
  to?: string;
}

function buildCrumbs(pathname: string): Crumb[] {
  const segments = pathname.split("/").filter(Boolean);
  const crumbs: Crumb[] = [{ label: "Home", to: "/" }];

  if (segments[0] === "locations") {
    crumbs.push({ label: "Locations", to: "/locations" });
    if (segments[1]) {
      const name = decodeURIComponent(segments[1]);
      crumbs.push({ label: name });
    }
  }

  return crumbs;
}

export function AppLayout({ children }: AppLayoutProps) {
  const location = useLocation();
  const navigation = useNavigation();
  const isNavigating = navigation.state !== "idle";
  const crumbs = buildCrumbs(location.pathname);
  const [sidebarOpen, setSidebarOpen] = useState(true);

  const pathSegments = location.pathname.split("/").filter(Boolean);
  const inLocationContext =
    pathSegments[0] === "locations" && pathSegments.length >= 2;
  const locationName = inLocationContext ? pathSegments[1] : null;

  return (
    <SidebarProvider open={sidebarOpen} onOpenChange={setSidebarOpen}>
      <Sidebar collapsible="icon">
        <SidebarHeader className="px-4 py-3 font-semibold text-sm">
          Controller Template
        </SidebarHeader>
        {inLocationContext ? (
          <SidebarContent>
            <SidebarGroup>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild>
                      <Link to="/locations">
                        <ArrowLeftIcon className="size-4" />
                        Locations
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
            <SidebarGroup>
              <SidebarGroupLabel className="truncate">
                {decodeURIComponent(locationName ?? "")}
              </SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild isActive>
                      <Link to={`/locations/${locationName}`}>
                        <LayoutDashboardIcon className="size-4" />
                        Overview
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          </SidebarContent>
        ) : (
          <SidebarContent>
            <SidebarGroup>
              <SidebarGroupLabel>Manage</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton
                      asChild
                      isActive={location.pathname.startsWith("/locations")}
                    >
                      <Link to="/locations">
                        <BoxIcon className="size-4" />
                        Locations
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          </SidebarContent>
        )}
      </Sidebar>
      <SidebarInset>
        <div
          className={`h-0.5 bg-primary transition-opacity duration-200 ${
            isNavigating ? "opacity-100" : "opacity-0"
          }`}
        />
        <header className="flex h-12 shrink-0 items-center gap-2 border-b border-border/50 px-4">
          <SidebarTrigger className="-ml-1" />
          <Breadcrumb>
            <BreadcrumbList>
              {crumbs.map((c, i) => {
                const isLast = i === crumbs.length - 1;
                return (
                  <span key={i} className="contents">
                    <BreadcrumbItem>
                      {isLast || !c.to ? (
                        <BreadcrumbPage>{c.label}</BreadcrumbPage>
                      ) : (
                        <BreadcrumbLink asChild>
                          <Link to={c.to}>{c.label}</Link>
                        </BreadcrumbLink>
                      )}
                    </BreadcrumbItem>
                    {!isLast && <BreadcrumbSeparator />}
                  </span>
                );
              })}
            </BreadcrumbList>
          </Breadcrumb>
        </header>
        <div className="flex-1 min-h-0 overflow-auto">{children}</div>
      </SidebarInset>
    </SidebarProvider>
  );
}
