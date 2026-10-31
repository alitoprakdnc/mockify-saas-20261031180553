import React, { useState, ChangeEvent, FormEvent } from "react";

type HttpMethod =
  | "GET"
  | "POST"
  | "PUT"
  | "PATCH"
  | "DELETE"
  | "OPTIONS"
  | "HEAD";

interface HeaderEntry {
  key: string;
  value: string;
}

interface MockResponse {
  status: number;
  statusText: string;
  headers: Record<string, string>;
  body: string;
  parsedBody?: unknown;
  error?: string;
}

interface MockServerViewerProps {
  /** Base URL of the generated mock server, e.g. https://mockify.com/mock/abcd1234 */
  mockBaseUrl: string;
  /** Optional initial path (including leading slash) */
  initialPath?: string;
}

/**
 * MockServerViewer
 *
 * Allows a user to craft an HTTP request against a mock endpoint and view the
 * raw response. The component is completely self‑contained – it does not rely
 * on any external state management and uses the native Fetch API.
 *
 * Production‑ready considerations:
 *  - Full TypeScript typing for request/response data.
 *  - Graceful error handling with user‑friendly messages.
 *  - Prevents accidental request loops by disabling the button while a request is in flight.
 *  - Normalises header names to lower‑case for deterministic display.
 *  - Attempts to pretty‑print JSON responses.
 */
const MockServerViewer: React.FC<MockServerViewerProps> = ({
  mockBaseUrl,
  initialPath = "/",
}) => {
  const [method, setMethod] = useState<HttpMethod>("GET");
  const [path, setPath] = useState<string>(initialPath);
  const [headers, setHeaders] = useState<HeaderEntry[]>([{ key: "", value: "" }]);
  const [body, setBody] = useState<string>("");
  const [response, setResponse] = useState<MockResponse | null>(null);
  const [loading, setLoading] = useState<boolean>(false);

  // -------------------------------------------------------------------------
  // Helper utilities
  // -------------------------------------------------------------------------

  const normalizeUrl = (base: string, p: string): string => {
    const trimmedBase = base.endsWith("/") ? base.slice(0, -1) : base;
    const trimmedPath = p.startsWith("/") ? p : `/${p}`;
    return `${trimmedBase}${trimmedPath}`;
  };

  const buildHeaders = (): Record<string, string> => {
    const result: Record<string, string> = {};
    headers.forEach(({ key, value }) => {
      const trimmedKey = key.trim();
      if (trimmedKey) {
        result[trimmedKey] = value;
      }
    });
    return result;
  };

  const isBodyAllowed = (m: HttpMethod): boolean =>
    ["POST", "PUT", "PATCH", "DELETE"].includes(m);

  // -------------------------------------------------------------------------
  // Event handlers
  // -------------------------------------------------------------------------

  const handleMethodChange = (e: ChangeEvent<HTMLSelectElement>) => {
    setMethod(e.target.value as HttpMethod);
  };

  const handlePathChange = (e: ChangeEvent<HTMLInputElement>) => {
    setPath(e.target.value);
  };

  const handleHeaderChange = (
    index: number,
    field: keyof HeaderEntry,
    value: string
  ) => {
    setHeaders((prev) => {
      const copy = [...prev];
      copy[index] = { ...copy[index], [field]: value };
      return copy;
    });
  };

  const handleAddHeader = () => {
    setHeaders((prev) => [...prev, { key: "", value: "" }]);
  };

  const handleRemoveHeader = (index: number) => {
    setHeaders((prev) => prev.filter((_, i) => i !== index));
  };

  const handleBodyChange = (e: ChangeEvent<HTMLTextAreaElement>) => {
    setBody(e.target.value);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();

    // Basic validation
    if (!path.trim()) {
      setResponse({
        status: 0,
        statusText: "Invalid request",
        headers: {},
        body: "",
        error: "Path cannot be empty.",
      });
      return;
    }

    const url = normalizeUrl(mockBaseUrl, path);
    const requestHeaders = buildHeaders();

    const fetchOptions: RequestInit = {
      method,
      headers: requestHeaders,
    };

    if (isBodyAllowed(method) && body) {
      fetchOptions.body = body;
    }

    setLoading(true);
    setResponse(null);

    try {
      const res = await fetch(url, fetchOptions);
      const rawHeaders: Record<string, string> = {};
      res.headers.forEach((value, key) => {
        rawHeaders[key] = value;
      });

      const textBody = await res.text();

      // Try to parse JSON for pretty printing
      let parsedBody: unknown | undefined;
      try {
        parsedBody = JSON.parse(textBody);
      } catch {
        // Not JSON – keep raw text
      }

      setResponse({
        status: res.status,
        statusText: res.statusText,
        headers: rawHeaders,
        body: textBody,
        parsedBody,
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Unknown error occurred";
      setResponse({
        status: 0,
        statusText: "Network error",
        headers: {},
        body: "",
        error: message,
      });
    } finally {
      setLoading(false);
    }
  };

  // -------------------------------------------------------------------------
  // Render helpers
  // -------------------------------------------------------------------------

  const renderResponse = () => {
    if (!response) return null;

    const { status, statusText, headers, body, parsedBody, error } = response;

    return (
      <div className="mock-response">
        <h3>Response</h3>
        {error ? (
          <div className="error-message">{error}</div>
        ) : (
          <>
            <div>
              <strong>Status:</strong> {status} {statusText}
            </div>
            <details className="response-headers">
              <summary>Headers ({Object.keys(headers).length})</summary>
              <pre>{JSON.stringify(headers, null, 2)}</pre>
            </details>
            <details className="response-body" open>
              <summary>Body</summary>
              <pre>
                {parsedBody
                  ? JSON.stringify(parsedBody, null, 2)
                  : body || "(empty)"}
              </pre>
            </details>
          </>
        )}
      </div>
    );
  };

  // -------------------------------------------------------------------------
  // JSX
  // -------------------------------------------------------------------------

  return (
    <section className="mock-server-viewer">
      <h2>Mock Server Explorer</h2>
      <form onSubmit={handleSubmit} className="mock-request-form">
        <div className="form-row">
          <label htmlFor="method">Method</label>
          <select
            id="method"
            value={method}
            onChange={handleMethodChange}
            disabled={loading}
          >
            {["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"].map(
              (m) => (
                <option key={m} value={m}>
                  {m}
                </option>
              )
            )}
          </select>
        </div>

        <div className="form-row">
          <label htmlFor="path">Path</label>
          <input
            id="path"
            type="text"
            value={path}
            onChange={handlePathChange}
            placeholder="/users"
            disabled={loading}
            required
          />
        </div>

        <fieldset className="headers-fieldset">
          <legend>Headers</legend>
          {headers.map((h, idx) => (
            <div key={idx} className="header-entry">
              <input
                type="text"
                placeholder="Key"
                value={h.key}
                onChange={(e) =>
                  handleHeaderChange(idx, "key", e.target.value)
                }
                disabled={loading}
              />
              <input
                type="text"
                placeholder="Value"
                value={h.value}
                onChange={(e) =>
                  handleHeaderChange(idx, "value", e.target.value)
                }
                disabled={loading}
              />
              <button
                type="button"
                onClick={() => handleRemoveHeader(idx)}
                disabled={loading || headers.length === 1}
                aria-label="Remove header"
                className="remove-header-btn"
              >
                ✕
              </button>
            </div>
          ))}
          <button
            type="button"
            onClick={handleAddHeader}
            disabled={loading}
            className="add-header-btn"
          >
            + Add Header
          </button>
        </fieldset>

        {isBodyAllowed(method) && (
          <div className="form-row">
            <label htmlFor="body">Body</label>
            <textarea
              id="body"
              rows={6}
              value={body}
              onChange={handleBodyChange}
              placeholder='{"name":"John"}'
              disabled={loading}
            />
          </div>
        )}

        <div className="form-actions">
          <button type="submit" disabled={loading} className="send-btn">
            {loading ? "Sending…" : "Send Request"}
          </button>
        </div>
      </form>

      {renderResponse()}
    </section>
  );
};

export default MockServerViewer;