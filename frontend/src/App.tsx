import React, { createContext, useContext, useEffect, useReducer, useCallback } from "react";
import {
  BrowserRouter,
  Routes,
  Route,
  Navigate,
  useNavigate,
  useParams,
} from "react-router-dom";
import axios, { AxiosError } from "axios";
import ContractList from "./components/ContractList";
import MockServerViewer from "./components/MockServerViewer";
import "./App.css";

/* -------------------------------------------------------------------------- */
/*                               Environment                                 */
/* -------------------------------------------------------------------------- */
const API_BASE_URL = process.env.REACT_APP_API_URL ?? "/api";

/* -------------------------------------------------------------------------- */
/*                               Auth Context                                 */
/* -------------------------------------------------------------------------- */
type User = {
  id: string;
  email: string;
  role: string;
};

type AuthState = {
  user: User | null;
  token: string | null;
};

type AuthAction =
  | { type: "LOGIN"; payload: { user: User; token: string } }
  | { type: "LOGOUT" };

const AuthContext = createContext<{
  state: AuthState;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
}>({
  state: { user: null, token: null },
  login: async () => {},
  logout: () => {},
});

const authReducer = (state: AuthState, action: AuthAction): AuthState => {
  switch (action.type) {
    case "LOGIN":
      return { user: action.payload.user, token: action.payload.token };
    case "LOGOUT":
      return { user: null, token: null };
    default:
      return state;
  }
};

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const [state, dispatch] = useReducer(authReducer, {
    user: null,
    token: null,
  });

  // Initialise from localStorage
  useEffect(() => {
    const storedToken = localStorage.getItem("mockify_token");
    const storedUser = localStorage.getItem("mockify_user");
    if (storedToken && storedUser) {
      dispatch({
        type: "LOGIN",
        payload: { token: storedToken, user: JSON.parse(storedUser) },
      });
    }
  }, []);

  const login = async (email: string, password: string) => {
    try {
      const resp = await axios.post(`${API_BASE_URL}/auth/login`, {
        email,
        password,
      });
      const { token, user } = resp.data;
      localStorage.setItem("mockify_token", token);
      localStorage.setItem("mockify_user", JSON.stringify(user));
      dispatch({ type: "LOGIN", payload: { token, user } });
    } catch (err) {
      const e = err as AxiosError;
      const msg = e.response?.data?.message ?? "Login failed";
      throw new Error(msg);
    }
  };

  const logout = () => {
    localStorage.removeItem("mockify_token");
    localStorage.removeItem("mockify_user");
    dispatch({ type: "LOGOUT" });
  };

  // Attach token to every request
  useEffect(() => {
    const interceptor = axios.interceptors.request.use((config) => {
      if (state.token) {
        config.headers.Authorization = `Bearer ${state.token}`;
      }
      return config;
    });
    return () => {
      axios.interceptors.request.eject(interceptor);
    };
  }, [state.token]);

  return (
    <AuthContext.Provider value={{ state, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => useContext(AuthContext);

/* -------------------------------------------------------------------------- */
/*                             Contracts Context                              */
/* -------------------------------------------------------------------------- */
export type Contract = {
  id: string;
  name: string;
  version: string;
  spec: object; // OpenAPI spec JSON
  createdAt: string;
  updatedAt: string;
};

type ContractsState = {
  contracts: Contract[];
  loading: boolean;
  error?: string;
};

type ContractsAction =
  | { type: "SET_LOADING"; payload: boolean }
  | { type: "SET_ERROR"; payload?: string }
  | { type: "SET_CONTRACTS"; payload: Contract[] }
  | { type: "ADD_CONTRACT"; payload: Contract }
  | { type: "UPDATE_CONTRACT"; payload: Contract }
  | { type: "DELETE_CONTRACT"; payload: string };

const ContractsContext = createContext<{
  state: ContractsState;
  fetchContracts: () => Promise<void>;
  addContract: (
    data: Omit<Contract, "id" | "createdAt" | "updatedAt">
  ) => Promise<void>;
  updateContract: (id: string, data: Partial<Contract>) => Promise<void>;
  deleteContract: (id: string) => Promise<void>;
}>({
  state: { contracts: [], loading: false },
  fetchContracts: async () => {},
  addContract: async () => {},
  updateContract: async () => {},
  deleteContract: async () => {},
});

const contractsReducer = (
  state: ContractsState,
  action: ContractsAction
): ContractsState => {
  switch (action.type) {
    case "SET_LOADING":
      return { ...state, loading: action.payload };
    case "SET_ERROR":
      return { ...state, error: action.payload };
    case "SET_CONTRACTS":
      return { ...state, contracts: action.payload };
    case "ADD_CONTRACT":
      return { ...state, contracts: [action.payload, ...state.contracts] };
    case "UPDATE_CONTRACT":
      return {
        ...state,
        contracts: state.contracts.map((c) =>
          c.id === action.payload.id ? action.payload : c
        ),
      };
    case "DELETE_CONTRACT":
      return {
        ...state,
        contracts: state.contracts.filter((c) => c.id !== action.payload),
      };
    default:
      return state;
  }
};

export const ContractsProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const [state, dispatch] = useReducer(contractsReducer, {
    contracts: [],
    loading: false,
  });

  const fetchContracts = useCallback(async () => {
    dispatch({ type: "SET_LOADING", payload: true });
    dispatch({ type: "SET_ERROR", payload: undefined });
    try {
      const resp = await axios.get<Contract[]>(`${API_BASE_URL}/contracts`);
      dispatch({ type: "SET_CONTRACTS", payload: resp.data });
    } catch (err) {
      const e = err as AxiosError;
      dispatch({
        type: "SET_ERROR",
        payload: e.response?.data?.message ?? "Failed to load contracts",
      });
    } finally {
      dispatch({ type: "SET_LOADING", payload: false });
    }
  }, []);

  const addContract = useCallback(
    async (
      data: Omit<Contract, "id" | "createdAt" | "updatedAt">
    ): Promise<void> => {
      try {
        const resp = await axios.post<Contract>(`${API_BASE_URL}/contracts`, data);
        dispatch({ type: "ADD_CONTRACT", payload: resp.data });
      } catch (err) {
        const e = err as AxiosError;
        throw new Error(e.response?.data?.message ?? "Failed to add contract");
      }
    },
    []
  );

  const updateContract = useCallback(
    async (id: string, data: Partial<Contract>): Promise<void> => {
      try {
        const resp = await axios.patch<Contract>(`${API_BASE_URL}/contracts/${id}`, data);
        dispatch({ type: "UPDATE_CONTRACT", payload: resp.data });
      } catch (err) {
        const e = err as AxiosError;
        throw new Error(e.response?.data?.message ?? "Failed to update contract");
      }
    },
    []
  );

  const deleteContract = useCallback(
    async (id: string): Promise<void> => {
      try {
        await axios.delete(`${API_BASE_URL}/contracts/${id}`);
        dispatch({ type: "DELETE_CONTRACT", payload: id });
      } catch (err) {
        const e = err as AxiosError;
        throw new Error(e.response?.data?.message ?? "Failed to delete contract");
      }
    },
    []
  );

  // Load contracts once after authentication
  const {
    state: authState,
  } = useAuth();

  useEffect(() => {
    if (authState.token) {
      fetchContracts();
    }
  }, [authState.token, fetchContracts]);

  return (
    <ContractsContext.Provider
      value={{
        state,
        fetchContracts,
        addContract,
        updateContract,
        deleteContract,
      }}
    >
      {children}
    </ContractsContext.Provider>
  );
};

export const useContracts = () => useContext(ContractsContext);

/* -------------------------------------------------------------------------- */
/*                               Layout Components                             */
/* -------------------------------------------------------------------------- */
const Header: React.FC = () => {
  const {
    state: { user },
    logout,
  } = useAuth();

  return (
    <header className="app-header">
      <h1>Mockify SaaS</h1>
      {user && (
        <div className="user-info">
          <span>{user.email}</span>
          <button onClick={logout} className="logout-btn">
            Logout
          </button>
        </div>
      )}
    </header>
  );
};

const ProtectedRoute: React.FC<{ children: JSX.Element }> = ({ children }) => {
  const {
    state: { token },
  } = useAuth();

  if (!token) {
    return <Navigate to="/login" replace />;
  }
  return children;
};

/* -------------------------------------------------------------------------- */
/*                               Page Components                               */
/* -------------------------------------------------------------------------- */
const Dashboard: React.FC = () => {
  return (
    <div className="page dashboard">
      <h2>Welcome to Mockify</h2>
      <p>
        Manage your OpenAPI contracts, generate mock servers, and integrate
        contract‑based testing into your CI/CD pipelines.
      </p>
    </div>
  );
};

const LoginPage: React.FC = () => {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [error, setError] = React.useState<string | null>(null);
  const [loading, setLoading] = React.useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await login(email, password);
      navigate("/", { replace: true });
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="page login-page">
      <h2>Login</h2>
      <form onSubmit={handleSubmit} className="login-form">
        {error && <div className="error">{error}</div>}
        <label>
          Email
          <input
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            disabled={loading}
          />
        </label>
        <label>
          Password
          <input
            type="password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={loading}
          />
        </label>
        <button type="submit" disabled={loading}>
          {loading ? "Logging in…" : "Login"}
        </button>
      </form>
    </div>
  );
};

const ContractDetailWrapper: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  if (!id) {
    return <Navigate to="/contracts" replace />;
  }

  return (
    <MockServerViewer
      contractId={id}
      onBack={() => navigate("/contracts")}
    />
  );
};

/* -------------------------------------------------------------------------- */
/*                                   App                                        */
/* -------------------------------------------------------------------------- */
const App: React.FC = () => {
  return (
    <AuthProvider>
      <ContractsProvider>
        <BrowserRouter>
          <Header />
          <main className="app-main">
            <Routes>
              <Route
                path="/login"
                element={<LoginPage />}
              />
              <Route
                path="/"
                element={
                  <ProtectedRoute>
                    <Dashboard />
                  </ProtectedRoute>
                }
              />
              <Route
                path="/contracts"
                element={
                  <ProtectedRoute>
                    <ContractList />
                  </ProtectedRoute>
                }
              />
              <Route
                path="/contracts/:id/mock"
                element={
                  <ProtectedRoute>
                    <ContractDetailWrapper />
                  </ProtectedRoute>
                }
              />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </main>
        </BrowserRouter>
      </ContractsProvider>
    </AuthProvider>
  );
};

export default App;