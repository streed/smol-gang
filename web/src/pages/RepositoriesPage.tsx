import { useEffect, useState, FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, Github } from 'lucide-react';
import { repos as reposApi, github as githubApi } from '../api/endpoints';
import Modal from '../components/Modal';
import Pagination from '../components/Pagination';
import type { Repository, GitHubRepo } from '../types';
import toast from 'react-hot-toast';

export default function RepositoriesPage() {
  const [repositories, setRepositories] = useState<Repository[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    github_owner: '',
    github_repo: '',
    git_url: '',
    default_branch: 'main',
  });
  const [submitting, setSubmitting] = useState(false);
  const [showGitHubModal, setShowGitHubModal] = useState(false);
  const [ghRepos, setGhRepos] = useState<GitHubRepo[]>([]);
  const [ghLoading, setGhLoading] = useState(false);
  const [ghError, setGhError] = useState('');
  const [importing, setImporting] = useState<string | null>(null);
  const navigate = useNavigate();

  const fetchRepos = async (p: number) => {
    setLoading(true);
    try {
      const res = await reposApi.listRepos(p);
      setRepositories(res.data.items || []);
      setTotal(res.data.total || 0);
    } catch {
      toast.error('Failed to load repositories');
    } finally {
      setLoading(false);
    }
  };

  const fetchGitHubRepos = async () => {
    setGhLoading(true);
    setGhError('');
    try {
      const res = await githubApi.listRepos();
      setGhRepos(res.data || []);
    } catch {
      setGhError('Failed to load GitHub repos. Make sure your GitHub account is linked.');
    } finally {
      setGhLoading(false);
    }
  };

  const handleImport = async (repo: GitHubRepo) => {
    setImporting(repo.full_name);
    try {
      await githubApi.importRepo(repo.owner, repo.name);
      toast.success(`Imported ${repo.full_name}`);
      fetchRepos(page);
      setShowGitHubModal(false);
    } catch {
      toast.error(`Failed to import ${repo.full_name}`);
    } finally {
      setImporting(null);
    }
  };

  const openGitHubModal = () => {
    setShowGitHubModal(true);
    fetchGitHubRepos();
  };

  useEffect(() => {
    fetchRepos(page);
  }, [page]);

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      await reposApi.createRepo(formData);
      toast.success('Repository linked successfully');
      setShowModal(false);
      setFormData({
        name: '',
        github_owner: '',
        github_repo: '',
        git_url: '',
        default_branch: 'main',
      });
      fetchRepos(page);
    } catch {
      toast.error('Failed to link repository');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-display font-bold text-gray-100 uppercase tracking-wider">Repositories</h1>
        <div className="flex items-center gap-2">
          <button
            onClick={openGitHubModal}
            className="btn-cyber flex items-center gap-2 border border-cyber-border text-gray-300 hover:border-gray-500 hover:text-white"
          >
            <Github className="h-4 w-4" />
            Import from GitHub
          </button>
          <button
            onClick={() => setShowModal(true)}
            className="btn-neon-cyan flex items-center gap-2"
          >
            <Plus className="h-4 w-4" />
            Link Repository
          </button>
        </div>
      </div>

      <div className="bg-cyber-card border border-cyber-border rounded-lg">
        <div className="overflow-x-auto">
          <table className="table-cyber w-full">
            <thead>
              <tr className="border-b border-cyber-border">
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Name
                </th>
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  GitHub
                </th>
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Branch
                </th>
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Created
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-cyber-border">
              {loading ? (
                <tr>
                  <td colSpan={4} className="px-6 py-8 text-center">
                    <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-neon-cyan mx-auto" />
                  </td>
                </tr>
              ) : repositories.length === 0 ? (
                <tr>
                  <td
                    colSpan={4}
                    className="px-6 py-8 text-center text-sm text-gray-500"
                  >
                    No repositories linked yet.
                  </td>
                </tr>
              ) : (
                repositories.map((repo) => (
                  <tr
                    key={repo.id}
                    onClick={() => navigate(`/repositories/${repo.id}`)}
                    className="hover:bg-neon-cyan/5 cursor-pointer transition-colors"
                  >
                    <td className="px-6 py-4 text-sm font-medium text-gray-300">
                      {repo.name}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-400">
                      {repo.github_owner}/{repo.github_repo}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-400">
                      {repo.default_branch}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-400">
                      {new Date(repo.created_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
        {total > 0 && (
          <div className="px-6 border-t border-cyber-border">
            <Pagination
              page={page}
              perPage={20}
              total={total}
              onPageChange={setPage}
            />
          </div>
        )}
      </div>

      {/* Link Repository Modal */}
      <Modal
        open={showModal}
        onClose={() => setShowModal(false)}
        title="Link Repository"
      >
        <form onSubmit={handleCreate} className="space-y-4">
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Name
            </label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) =>
                setFormData({ ...formData, name: e.target.value })
              }
              required
              className="input-cyber w-full"
              placeholder="my-project"
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-mono text-gray-400 mb-1">
                GitHub Owner
              </label>
              <input
                type="text"
                value={formData.github_owner}
                onChange={(e) =>
                  setFormData({ ...formData, github_owner: e.target.value })
                }
                required
                className="input-cyber w-full"
                placeholder="owner"
              />
            </div>
            <div>
              <label className="block text-sm font-mono text-gray-400 mb-1">
                GitHub Repo
              </label>
              <input
                type="text"
                value={formData.github_repo}
                onChange={(e) =>
                  setFormData({ ...formData, github_repo: e.target.value })
                }
                required
                className="input-cyber w-full"
                placeholder="repo"
              />
            </div>
          </div>
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Git URL
            </label>
            <input
              type="text"
              value={formData.git_url}
              onChange={(e) =>
                setFormData({ ...formData, git_url: e.target.value })
              }
              required
              className="input-cyber w-full"
              placeholder="https://github.com/owner/repo.git"
            />
          </div>
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Default Branch
            </label>
            <input
              type="text"
              value={formData.default_branch}
              onChange={(e) =>
                setFormData({ ...formData, default_branch: e.target.value })
              }
              className="input-cyber w-full"
              placeholder="main"
            />
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={() => setShowModal(false)}
              className="btn-cyber text-gray-400 border-cyber-border hover:text-gray-200"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="btn-neon-cyan disabled:opacity-50"
            >
              {submitting ? 'Linking...' : 'Link Repository'}
            </button>
          </div>
        </form>
      </Modal>

      {/* Import from GitHub Modal */}
      <Modal
        open={showGitHubModal}
        onClose={() => setShowGitHubModal(false)}
        title="Import from GitHub"
      >
        <div className="max-h-96 overflow-y-auto">
          {ghLoading ? (
            <div className="flex justify-center py-8">
              <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-neon-cyan" />
            </div>
          ) : ghError ? (
            <p className="text-neon-red text-sm font-mono py-4">{ghError}</p>
          ) : ghRepos.length === 0 ? (
            <p className="text-gray-500 text-sm font-mono py-4">No repositories found.</p>
          ) : (
            <div className="space-y-2">
              {ghRepos.map((repo) => (
                <div
                  key={repo.full_name}
                  className="flex items-center justify-between p-3 border border-cyber-border rounded-lg hover:border-gray-600 transition-colors"
                >
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-gray-300 truncate">
                      {repo.full_name}
                    </p>
                    {repo.description && repo.description !== 'null' && (
                      <p className="text-xs text-gray-500 truncate mt-0.5">
                        {repo.description}
                      </p>
                    )}
                    <p className="text-xs text-gray-600 mt-0.5">
                      {repo.default_branch} {repo.private ? '(private)' : ''}
                    </p>
                  </div>
                  <button
                    onClick={() => handleImport(repo)}
                    disabled={importing === repo.full_name}
                    className="btn-neon-cyan text-xs ml-3 flex-shrink-0"
                  >
                    {importing === repo.full_name ? 'Importing...' : 'Import'}
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      </Modal>
    </div>
  );
}
