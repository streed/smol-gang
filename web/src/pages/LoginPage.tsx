import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { auth as authApi } from '../api/endpoints';
import { Cpu } from 'lucide-react';

export default function LoginPage() {
  const [searchParams] = useSearchParams();
  const [error, setError] = useState(searchParams.get('error') || '');
  const [loading, setLoading] = useState(false);

  const handleGitHubLogin = async () => {
    setError('');
    setLoading(true);
    try {
      const res = await authApi.getGitHubLoginUrl();
      window.location.href = res.data.url;
    } catch {
      setError('GitHub login is not configured');
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-cyber-bg px-4 relative overflow-hidden">
      {/* Grid pattern background */}
      <div
        className="absolute inset-0 opacity-10"
        style={{
          backgroundImage:
            'linear-gradient(rgba(0,255,255,0.1) 1px, transparent 1px), linear-gradient(90deg, rgba(0,255,255,0.1) 1px, transparent 1px)',
          backgroundSize: '40px 40px',
        }}
      />

      {/* Scanlines overlay */}
      <div
        className="absolute inset-0 pointer-events-none z-10"
        style={{
          background:
            'repeating-linear-gradient(0deg, transparent, transparent 2px, rgba(0,0,0,0.15) 2px, rgba(0,0,0,0.15) 4px)',
        }}
      />

      <div className="w-full max-w-md relative z-20">
        {/* Logo / Branding */}
        <div className="flex flex-col items-center mb-8">
          <div className="flex items-center gap-3 mb-2">
            <Cpu className="h-10 w-10 text-neon-cyan drop-shadow-[0_0_8px_rgba(0,255,255,0.6)]" />
            <h1 className="text-3xl font-display tracking-widest">
              <span className="text-neon-cyan drop-shadow-[0_0_10px_rgba(0,255,255,0.5)]">
                SMOL
              </span>
              <span className="text-gray-200">GANG</span>
            </h1>
          </div>
          <p className="text-gray-400 text-sm font-mono tracking-wider">
            Kubernetes Agent Orchestration
          </p>
        </div>

        {/* Login card */}
        <div className="cyber-card bg-cyber-card border border-cyber-border rounded-lg p-8 shadow-[0_0_30px_rgba(0,255,255,0.05)]">
          <h2 className="text-xl font-display text-gray-200 mb-6 tracking-wide">
            // AUTHENTICATE
          </h2>

          {error && (
            <div className="mb-4 p-3 bg-neon-red/10 border border-neon-red/30 rounded-lg text-sm text-neon-red font-mono">
              {error}
            </div>
          )}

          <button
            onClick={handleGitHubLogin}
            disabled={loading}
            className="btn-neon-cyan w-full py-3 font-mono tracking-wider uppercase flex items-center justify-center gap-2"
          >
            <svg className="h-5 w-5" fill="currentColor" viewBox="0 0 24 24"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
            {loading ? '> Connecting to GitHub...' : '> Sign in with GitHub'}
          </button>
        </div>

        {/* Footer accent */}
        <p className="text-center text-gray-600 text-xs font-mono mt-6 tracking-widest uppercase">
          Secure Terminal v2.0
        </p>
      </div>
    </div>
  );
}
