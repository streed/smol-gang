import { useEffect, useState, FormEvent } from 'react';
import { Plus, Trash2, Pencil } from 'lucide-react';
import { users as usersApi, auth as authApi } from '../api/endpoints';
import Modal from '../components/Modal';
import Pagination from '../components/Pagination';
import type { User } from '../types';
import toast from 'react-hot-toast';

export default function UsersPage() {
  const [usersList, setUsersList] = useState<User[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);

  // Add user modal
  const [showAddModal, setShowAddModal] = useState(false);
  const [addForm, setAddForm] = useState({
    email: '',
    name: '',
    password: '',
    role: 'user',
  });
  const [submitting, setSubmitting] = useState(false);

  // Edit user modal
  const [showEditModal, setShowEditModal] = useState(false);
  const [editUser, setEditUser] = useState<User | null>(null);
  const [editRole, setEditRole] = useState('user');

  // Delete confirmation
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<User | null>(null);

  const fetchUsers = async (p: number) => {
    setLoading(true);
    try {
      const res = await usersApi.listUsers(p);
      setUsersList(res.data.items || []);
      setTotal(res.data.total || 0);
    } catch {
      toast.error('Failed to load users');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchUsers(page);
  }, [page]);

  const handleAddUser = async (e: FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      await authApi.register(
        addForm.email,
        addForm.password,
        addForm.name,
        addForm.role
      );
      toast.success('User created');
      setShowAddModal(false);
      setAddForm({ email: '', name: '', password: '', role: 'user' });
      fetchUsers(page);
    } catch {
      toast.error('Failed to create user');
    } finally {
      setSubmitting(false);
    }
  };

  const handleEditUser = async (e: FormEvent) => {
    e.preventDefault();
    if (!editUser) return;
    setSubmitting(true);
    try {
      await usersApi.updateUser(editUser.id, { role: editRole });
      toast.success('User updated');
      setShowEditModal(false);
      fetchUsers(page);
    } catch {
      toast.error('Failed to update user');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeleteUser = async () => {
    if (!deleteTarget) return;
    try {
      await usersApi.deleteUser(deleteTarget.id);
      toast.success('User deleted');
      setShowDeleteModal(false);
      setDeleteTarget(null);
      fetchUsers(page);
    } catch {
      toast.error('Failed to delete user');
    }
  };

  const roleBadgeClass = (role: string) => {
    switch (role) {
      case 'admin':
        return 'bg-neon-magenta/10 text-neon-magenta border border-neon-magenta/30';
      case 'operator':
        return 'bg-neon-cyan/10 text-neon-cyan border border-neon-cyan/30';
      default:
        return 'bg-gray-500/10 text-gray-400 border border-gray-500/30';
    }
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-display font-bold text-gray-100 uppercase tracking-wider">Users</h1>
        <button
          onClick={() => setShowAddModal(true)}
          className="btn-neon-cyan flex items-center gap-2"
        >
          <Plus className="h-4 w-4" />
          Add User
        </button>
      </div>

      <div className="bg-cyber-card border border-cyber-border rounded-lg">
        <div className="overflow-x-auto">
          <table className="table-cyber w-full">
            <thead>
              <tr className="border-b border-cyber-border">
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Email
                </th>
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Name
                </th>
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Role
                </th>
                <th className="text-left text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Created
                </th>
                <th className="text-right text-xs font-medium text-gray-500 uppercase tracking-wider px-6 py-3">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-cyber-border">
              {loading ? (
                <tr>
                  <td colSpan={5} className="px-6 py-8 text-center">
                    <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-neon-cyan mx-auto" />
                  </td>
                </tr>
              ) : usersList.length === 0 ? (
                <tr>
                  <td
                    colSpan={5}
                    className="px-6 py-8 text-center text-sm text-gray-500"
                  >
                    No users found.
                  </td>
                </tr>
              ) : (
                usersList.map((u) => (
                  <tr key={u.id} className="hover:bg-neon-cyan/5 transition-colors">
                    <td className="px-6 py-4 text-sm text-gray-300">
                      {u.email}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-300">
                      {u.name}
                    </td>
                    <td className="px-6 py-4">
                      <span
                        className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium capitalize ${roleBadgeClass(u.role)}`}
                      >
                        {u.role}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-400">
                      {new Date(u.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-6 py-4 text-right">
                      <div className="flex items-center justify-end gap-2">
                        <button
                          onClick={() => {
                            setEditUser(u);
                            setEditRole(u.role);
                            setShowEditModal(true);
                          }}
                          className="p-1.5 text-gray-500 hover:text-neon-cyan hover:bg-cyber-hover rounded-lg transition-colors"
                          title="Edit"
                        >
                          <Pencil className="h-4 w-4" />
                        </button>
                        <button
                          onClick={() => {
                            setDeleteTarget(u);
                            setShowDeleteModal(true);
                          }}
                          className="p-1.5 text-gray-500 hover:text-neon-red hover:bg-cyber-hover rounded-lg transition-colors"
                          title="Delete"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </div>
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

      {/* Add User Modal */}
      <Modal
        open={showAddModal}
        onClose={() => setShowAddModal(false)}
        title="Add User"
      >
        <form onSubmit={handleAddUser} className="space-y-4">
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Email
            </label>
            <input
              type="email"
              value={addForm.email}
              onChange={(e) =>
                setAddForm({ ...addForm, email: e.target.value })
              }
              required
              className="input-cyber w-full"
            />
          </div>
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Name
            </label>
            <input
              type="text"
              value={addForm.name}
              onChange={(e) =>
                setAddForm({ ...addForm, name: e.target.value })
              }
              required
              className="input-cyber w-full"
            />
          </div>
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Password
            </label>
            <input
              type="password"
              value={addForm.password}
              onChange={(e) =>
                setAddForm({ ...addForm, password: e.target.value })
              }
              required
              minLength={8}
              className="input-cyber w-full"
            />
          </div>
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Role
            </label>
            <select
              value={addForm.role}
              onChange={(e) =>
                setAddForm({ ...addForm, role: e.target.value })
              }
              className="input-cyber w-full"
            >
              <option value="user">User</option>
              <option value="admin">Admin</option>
            </select>
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={() => setShowAddModal(false)}
              className="btn-cyber text-gray-400 border-cyber-border hover:text-gray-200"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="btn-neon-cyan disabled:opacity-50"
            >
              {submitting ? 'Creating...' : 'Add User'}
            </button>
          </div>
        </form>
      </Modal>

      {/* Edit User Modal */}
      <Modal
        open={showEditModal}
        onClose={() => setShowEditModal(false)}
        title="Edit User"
      >
        <form onSubmit={handleEditUser} className="space-y-4">
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Email
            </label>
            <input
              type="email"
              value={editUser?.email || ''}
              disabled
              className="bg-cyber-bg/50 border-cyber-border text-gray-500 w-full px-4 py-2.5 border rounded-lg text-sm"
            />
          </div>
          <div>
            <label className="block text-sm font-mono text-gray-400 mb-1">
              Role
            </label>
            <select
              value={editRole}
              onChange={(e) => setEditRole(e.target.value)}
              className="input-cyber w-full"
            >
              <option value="user">User</option>
              <option value="admin">Admin</option>
            </select>
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={() => setShowEditModal(false)}
              className="btn-cyber text-gray-400 border-cyber-border hover:text-gray-200"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="btn-neon-cyan disabled:opacity-50"
            >
              {submitting ? 'Saving...' : 'Save Changes'}
            </button>
          </div>
        </form>
      </Modal>

      {/* Delete Confirmation Modal */}
      <Modal
        open={showDeleteModal}
        onClose={() => setShowDeleteModal(false)}
        title="Delete User"
      >
        <p className="text-sm text-gray-400 mb-6">
          Are you sure you want to delete{' '}
          <span className="text-neon-red font-semibold">{deleteTarget?.email}</span>? This
          action cannot be undone.
        </p>
        <div className="flex justify-end gap-3">
          <button
            onClick={() => setShowDeleteModal(false)}
            className="btn-cyber text-gray-400 border-cyber-border hover:text-gray-200"
          >
            Cancel
          </button>
          <button
            onClick={handleDeleteUser}
            className="btn-neon-red"
          >
            Delete
          </button>
        </div>
      </Modal>
    </div>
  );
}
