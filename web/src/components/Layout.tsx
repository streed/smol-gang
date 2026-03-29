import { NavLink, Outlet } from 'react-router-dom';
import { useAuthStore } from '../store/auth';
import {
  LayoutDashboard,
  GitFork,
  Layers,
  Network,
  Users,
  ScrollText,
  LogOut,
  Cpu,
  Activity,
} from 'lucide-react';

const navItems = [
  { to: '/orchestrator', label: 'Orchestrator', icon: Network },
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/repositories', label: 'Repositories', icon: GitFork },
  { to: '/workstreams', label: 'Workstreams', icon: Layers },
  { to: '/plans', label: 'Plans', icon: ScrollText },
];

const adminItems = [
  { to: '/users', label: 'Users', icon: Users },
  { to: '/audit-log', label: 'Audit Log', icon: ScrollText },
];

export default function Layout_() {
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);
  const isAdmin = user?.role === 'admin';

  return (
    <div className="flex h-screen overflow-hidden bg-cyber-bg">
      {/* Sidebar */}
      <aside className="w-64 bg-cyber-surface border-r border-cyber-border flex flex-col flex-shrink-0 relative">
        {/* Subtle grid pattern */}
        <div className="absolute inset-0 bg-grid-pattern bg-grid opacity-30 pointer-events-none" />

        {/* Logo */}
        <div className="relative flex items-center gap-3 px-5 py-5 border-b border-cyber-border">
          <div className="relative">
            <Cpu className="h-7 w-7 text-neon-cyan" />
            <div className="absolute inset-0 animate-pulse-slow">
              <Cpu className="h-7 w-7 text-neon-cyan opacity-50 blur-sm" />
            </div>
          </div>
          <div>
            <span className="text-lg font-display font-bold text-neon-cyan text-glow-cyan tracking-wider">
              SMOL
            </span>
            <span className="text-lg font-display font-bold text-gray-400 tracking-wider">
              -CLUSTER
            </span>
          </div>
        </div>

        {/* System status */}
        <div className="relative px-5 py-3 border-b border-cyber-border">
          <div className="flex items-center gap-2">
            <Activity className="h-3 w-3 text-neon-green animate-pulse" />
            <span className="text-[10px] font-mono text-neon-green/80 uppercase tracking-[0.15em]">
              System Online
            </span>
          </div>
        </div>

        <nav className="relative flex-1 px-3 py-4 space-y-1 overflow-y-auto">
          <div className="px-3 pb-2">
            <p className="text-[10px] font-mono text-gray-600 uppercase tracking-[0.2em]">
              Navigation
            </p>
          </div>
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2.5 rounded text-sm font-mono transition-all duration-200 ${
                  isActive
                    ? 'bg-neon-cyan/10 text-neon-cyan border border-neon-cyan/30 shadow-neon-cyan'
                    : 'text-gray-400 hover:text-neon-cyan hover:bg-cyber-hover border border-transparent'
                }`
              }
            >
              <item.icon className="h-4 w-4" />
              {item.label}
            </NavLink>
          ))}

          {isAdmin && (
            <>
              <div className="pt-6 pb-2 px-3">
                <p className="text-[10px] font-mono text-gray-600 uppercase tracking-[0.2em]">
                  Admin
                </p>
              </div>
              {adminItems.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  className={({ isActive }) =>
                    `flex items-center gap-3 px-3 py-2.5 rounded text-sm font-mono transition-all duration-200 ${
                      isActive
                        ? 'bg-neon-magenta/10 text-neon-magenta border border-neon-magenta/30 shadow-neon-magenta'
                        : 'text-gray-400 hover:text-neon-magenta hover:bg-cyber-hover border border-transparent'
                    }`
                  }
                >
                  <item.icon className="h-4 w-4" />
                  {item.label}
                </NavLink>
              ))}
            </>
          )}
        </nav>

        {/* User info */}
        <div className="relative border-t border-cyber-border px-4 py-4">
          <div className="flex items-center gap-3">
            <div className="h-8 w-8 rounded bg-neon-cyan/10 border border-neon-cyan/30 flex items-center justify-center">
              <span className="text-xs font-mono font-bold text-neon-cyan">
                {user?.name?.charAt(0).toUpperCase()}
              </span>
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-mono text-gray-300 truncate">{user?.name}</p>
              <span className="text-[10px] font-mono text-neon-purple uppercase tracking-wider">
                {user?.role}
              </span>
            </div>
            <button
              onClick={logout}
              className="p-1.5 text-gray-500 hover:text-neon-red rounded transition-colors"
              title="Logout"
            >
              <LogOut className="h-4 w-4" />
            </button>
          </div>
        </div>
      </aside>

      {/* Main Area */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Top Bar */}
        <header className="h-12 bg-cyber-surface/50 backdrop-blur-sm border-b border-cyber-border flex items-center justify-between px-6 flex-shrink-0">
          <div className="flex items-center gap-2">
            <div className="h-1.5 w-1.5 rounded-full bg-neon-green animate-pulse" />
            <span className="text-[10px] font-mono text-gray-500 uppercase tracking-[0.15em]">
              {new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })} UTC
            </span>
          </div>
          <div className="text-[10px] font-mono text-gray-600">
            v0.1.0 // K8s Agent Orchestration
          </div>
        </header>

        {/* Content */}
        <main className="flex-1 overflow-y-auto bg-cyber-bg p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
