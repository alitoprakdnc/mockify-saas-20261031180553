import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import App from './App';
import './index.css';

/**
 * Global error boundary to catch unexpected errors in the React tree.
 * In production we log the error and render a fallback UI.
 */
class ErrorBoundary extends React.Component<
  { children: React.ReactNode },
  { hasError: boolean }
> {
  constructor(props: { children: React.ReactNode }) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(_: Error) {
    return { hasError: true };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    // Replace this with a real logging service in production
    console.error('Uncaught error:', error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div style={{ padding: '2rem', textAlign: 'center' }}>
          <h1>Something went wrong.</h1>
          <p>Please try refreshing the page or contact support if the problem persists.</p>
        </div>
      );
    }

    return this.props.children;
  }
}

/**
 * Configure React Query client with sensible defaults for a SaaS product.
 */
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Do not retry on failure by default; UI can decide per request
      retry: false,
      // Keep data fresh for 5 minutes
      staleTime: 5 * 60 * 1000,
      // Refetch on window focus to keep UI up‑to‑date
      refetchOnWindowFocus: true,
    },
    mutations: {
      retry: false,
    },
  },
});

/**
 * Bootstrap the React application.
 */
const rootElement = document.getElementById('root');

if (!rootElement) {
  throw new Error('Root element with id "root" not found.');
}

const root = ReactDOM.createRoot(rootElement);

root.render(
  <React.StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <App />
        </BrowserRouter>
      </QueryClientProvider>
    </ErrorBoundary>
  </React.StrictMode>
);