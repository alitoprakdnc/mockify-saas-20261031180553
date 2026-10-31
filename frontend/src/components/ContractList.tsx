import React, { useEffect, useState, useCallback } from "react";
import { useNavigate } from "react-router-dom";

type Contract = {
  id: string;
  name: string;
  version: string;
  description?: string;
  createdAt: string; // ISO string
  updatedAt: string; // ISO string
};

type ContractsResponse = {
  data: Contract[];
  total: number;
};

const API_BASE = process.env.REACT_APP_API_URL ?? "";

const fetchJSON = async <T,>(input: RequestInfo, init?: RequestInit): Promise<T> => {
  const response = await fetch(input, {
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
    },
    ...init,
  });

  if (!response.ok) {
    const errorBody = await response.text();
    throw new Error(`HTTP ${response.status}: ${errorBody}`);
  }

  return response.json();
};

export default function ContractList() {
  const navigate = useNavigate();

  const [contracts, setContracts] = useState<Contract[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [page, setPage] = useState<number>(1);
  const [pageSize] = useState<number>(10);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const loadContracts = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const url = `${API_BASE}/contracts?page=${page}&size=${pageSize}`;
      const result = await fetchJSON<ContractsResponse>(url);
      setContracts(result.data);
      setTotal(result.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [page, pageSize]);

  useEffect(() => {
    loadContracts();
  }, [loadContracts]);

  const handleView = (id: string) => {
    navigate(`/contracts/${id}`);
  };

  const handleEdit = (id: string) => {
    navigate(`/contracts/${id}/edit`);
  };

  const handleDelete = async (id: string) => {
    const confirmed = window.confirm(
      "Are you sure you want to delete this contract? This action cannot be undone."
    );
    if (!confirmed) return;

    try {
      await fetchJSON<void>(`${API_BASE}/contracts/${id}`, { method: "DELETE" });
      // Refresh list after deletion
      if (contracts.length === 1 && page > 1) {
        setPage(page - 1);
      } else {
        loadContracts();
      }
    } catch (err) {
      alert(`Failed to delete contract: ${err instanceof Error ? err.message : err}`);
    }
  };

  const handleGenerateMock = async (id: string) => {
    try {
      const result = await fetchJSON<{ mockUrl: string }>(`${API_BASE}/contracts/${id}/mock`, {
        method: "POST",
      });
      // Navigate to the mock viewer page, passing the generated URL via state
      navigate(`/mock/${id}`, { state: { mockUrl: result.mockUrl } });
    } catch (err) {
      alert(`Failed to generate mock server: ${err instanceof Error ? err.message : err}`);
    }
  };

  const totalPages = Math.ceil(total / pageSize);

  const renderPagination = () => {
    if (totalPages <= 1) return null;

    const pages = [];
    for (let i = 1; i <= totalPages; i++) {
      pages.push(
        <button
          key={i}
          onClick={() => setPage(i)}
          disabled={i === page}
          className={`mx-1 px-2 py-1 rounded ${
            i === page ? "bg-blue-600 text-white" : "bg-gray-200 hover:bg-gray-300"
          }`}
        >
          {i}
        </button>
      );
    }

    return (
      <div className="flex items-center justify-center mt-4">
        <button
          onClick={() => setPage((p) => Math.max(p - 1, 1))}
          disabled={page === 1}
          className="mx-1 px-2 py-1 rounded bg-gray-200 hover:bg-gray-300 disabled:opacity-50"
        >
          Prev
        </button>
        {pages}
        <button
          onClick={() => setPage((p) => Math.min(p + 1, totalPages))}
          disabled={page === totalPages}
          className="mx-1 px-2 py-1 rounded bg-gray-200 hover:bg-gray-3
          00 disabled:opacity-50"
        >
          Next
        </button>
      </div>
    );
  };

  return (
    <div className="p-6">
      <header className="flex justify-between items-center mb-4">
        <h1 className="text-2xl font-semibold">API Contracts</h1>
        <button
          onClick={() => navigate("/contracts/new")}
          className="bg-green-600 text-white px-4 py-2 rounded hover:bg-green-700"
        >
          New Contract
        </button>
      </header>

      {error && (
        <div className="mb-4 p-3 bg-red-100 text-red-800 rounded">
          Error: {error}
        </div>
      )}

      {loading ? (
        <div className="text-center py-8">Loading contracts…</div>
      ) : contracts.length === 0 ? (
        <div className="text-center py-8 text-gray-600">No contracts found.</div>
      ) : (
        <table className="min-w-full table-auto border-collapse">
          <thead className="bg-gray-100">
            <tr>
              <th className="px-4 py-2 text-left">Name</th>
              <th className="px-4 py-2 text-left">Version</th>
              <th className="px-4 py-2 text-left">Description</th>
              <th className="px-4 py-2 text-left">Created</th>
              <th className="px-4 py-2 text-left">Updated</th>
              <th className="px-4 py-2 text-center">Actions</th>
            </tr>
          </thead>
          <tbody>
            {contracts.map((c) => (
              <tr key={c.id} className="border-b hover:bg-gray-50">
                <td className="px-4 py-2">{c.name}</td>
                <td className="px-4 py-2">{c.version}</td>
                <td className="px-4 py-2">{c.description ?? "-"}</td>
                <td className="px-4 py-2">
                  {new Date(c.createdAt).toLocaleString()}
                </td>
                <td className="px-4 py-2">
                  {new Date(c.updatedAt).toLocaleString()}
                </td>
                <td className="px-4 py-2 text-center space-x-2">
                  <button
                    onClick={() => handleView(c.id)}
                    className="text-blue-600 hover:underline"
                  >
                    View
                  </button>
                  <button
                    onClick={() => handleEdit(c.id)}
                    className="text-indigo-600 hover:underline"
                  >
                    Edit
                  </button>
                  <button
                    onClick={() => handleGenerateMock(c.id)}
                    className="text-green-600 hover:underline"
                  >
                    Mock
                  </button>
                  <button
                    onClick={() => handleDelete(c.id)}
                    className="text-red-600 hover:underline"
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {renderPagination()}
    </div>
  );
}