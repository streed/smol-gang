import { useState, FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '../store/auth';
import { Cpu } from 'lucide-react';

export default function LoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const login = useAuthStore((s) => s.login);
  const navigate = useNavigate();

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await login(email, password);
      navigate('/dashboard');
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data
          ?.error || 'Login failed. Please check your credentials.';
      setError(msg);
    } finally {
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
              <span className="text-gray-200">-GANG</span>
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

          <form onSubmit={handleSubmit} className="space-y-5">
            <div>
              <label className="block text-sm font-mono text-gray-400 mb-1.5 tracking-wider uppercase">
                Email
              </label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                className="input-cyber w-full"
                placeholder="operator@smol-gang.io"
              />
            </div>
            <div>
              <label className="block text-sm font-mono text-gray-400 mb-1.5 tracking-wider uppercase">
                Password
              </label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                className="input-cyber w-full"
                placeholder="Enter your password"
              />
            </div>
            <button
              type="submit"
              disabled={loading}
              className="btn-neon-cyan w-full py-2.5 font-mono tracking-wider uppercase"
            >
              {loading ? '> Authenticating...' : '> Sign In'}
            </button>
          </form>
        </div>

        {/* Footer accent */}
        <p className="text-center text-gray-600 text-xs font-mono mt-6 tracking-widest uppercase">
          Secure Terminal v2.0
        </p>
      </div>
    </div>
  );
}
