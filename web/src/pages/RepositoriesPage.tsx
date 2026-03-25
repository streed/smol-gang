import { useEffect, useState, FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus } from 'lucide-react';
import { repos as reposApi } from '../api/endpoints';
import Modal from '../components/Modal';
import Pagination from '../components/Pagination';
import type { Repository } from '../types';
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
        <h1 className="text-2xl font-bold text-gray-900">Repositories</h1>
        <button
          onClick={() => setShowModal(true)}
          className="flex items-center gap-2 px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 transition-colors"
        >
          <Plus className="h-4 w-4" />
          Link Repository
        </button>
      </div>

      <div className="bg-white rounded-xl border border-gray-200">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-100">
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
            <tbody className="divide-y divide-gray-100">
              {loading ? (
                <tr>
                  <td colSpan={4} className="px-6 py-8 text-center">
                    <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-indigo-600 mx-auto" />
                  </td>
                </tr>
              ) : repositories.length === 0 ? (
                <tr>
                  <td
                    colSpan={4}
                    className="px-6 py-8 text-center text-sm text-gray-400"
                  >
                    No repositories linked yet.
                  </td>
                </tr>
              ) : (
                repositories.map((repo) => (
                  <tr
                    key={repo.id}
                    onClick={() => navigate(`/repositories/${repo.id}`)}
                    className="hover:bg-gray-50 cursor-pointer transition-colors"
                  >
                    <td className="px-6 py-4 text-sm font-medium text-gray-900">
                      {repo.name}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {repo.github_owner}/{repo.github_repo}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {repo.default_branch}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {new Date(repo.created_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
        {total > 0 && (
          <div className="px-6 border-t border-gray-100">
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
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Name
            </label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) =>
                setFormData({ ...formData, name: e.target.value })
              }
              required
              className="w-full px-4 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
              placeholder="my-project"
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                GitHub Owner
              </label>
              <input
                type="text"
                value={formData.github_owner}
                onChange={(e) =>
                  setFormData({ ...formData, github_owner: e.target.value })
                }
                required
                className="w-full px-4 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
                placeholder="owner"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                GitHub Repo
              </label>
              <input
                type="text"
                value={formData.github_repo}
                onChange={(e) =>
                  setFormData({ ...formData, github_repo: e.target.value })
                }
                required
                className="w-full px-4 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
                placeholder="repo"
              />
            </div>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Git URL
            </label>
            <input
              type="text"
              value={formData.git_url}
              onChange={(e) =>
                setFormData({ ...formData, git_url: e.target.value })
              }
              required
              className="w-full px-4 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
              placeholder="https://github.com/owner/repo.git"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Default Branch
            </label>
            <input
              type="text"
              value={formData.default_branch}
              onChange={(e) =>
                setFormData({ ...formData, default_branch: e.target.value })
              }
              className="w-full px-4 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
              placeholder="main"
            />
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={() => setShowModal(false)}
              className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="px-4 py-2 text-sm font-medium text-white bg-indigo-600 rounded-lg hover:bg-indigo-700 disabled:opacity-50"
            >
              {submitting ? 'Linking...' : 'Link Repository'}
            </button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
