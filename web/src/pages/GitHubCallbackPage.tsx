import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuthStore } from '../store/auth';
import { Cpu } from 'lucide-react';

export default function GitHubCallbackPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const loginWithToken = useAuthStore((s) => s.loginWithToken);
  const [error, setError] = useState('');

  useEffect(() => {
    const token = searchParams.get('token');
    const errorParam = searchParams.get('error');

    if (errorParam) {
      setError(`GitHub login failed: ${errorParam}`);
      return;
    }

    if (!token) {
      setError('No authentication token received');
      return;
    }

    loginWithToken(token)
      .then(() => navigate('/dashboard'))
      .catch(() => setError('Authentication failed'));
  }, [searchParams, loginWithToken, navigate]);

  return (
    <div className="min-h-screen flex items-center justify-center bg-cyber-bg px-4">
      <div className="text-center">
        <Cpu className="h-10 w-10 text-neon-cyan mx-auto mb-4 animate-pulse" />
        {error ? (
          <div>
            <p className="text-neon-red font-mono mb-4">{error}</p>
            <a href="/login" className="text-neon-cyan font-mono hover:underline">
              Back to login
            </a>
          </div>
        ) : (
          <p className="text-gray-400 font-mono">Authenticating with GitHub...</p>
        )}
      </div>
    </div>
  );
}
