import { useEffect, useState } from "react";
import { ApiError, listSubmissions, runCode } from "../api";
import type { Language, Submission } from "../types";

const STARTER: Record<Language, string> = {
  python: 'print("hello from python")',
  node: 'console.log("hello from node");',
  go: 'package main\n\nimport "fmt"\n\nfunc main() {\n\tfmt.Println("hello from go")\n}\n',
};

export default function RunPanel({ token, onExpired }: { token: string; onExpired: () => void }) {
  const [language, setLanguage] = useState<Language>("python");
  const [code, setCode] = useState(STARTER.python);
  const [result, setResult] = useState<Submission | null>(null);
  const [history, setHistory] = useState<Submission[]>([]);
  const [error, setError] = useState("");
  const [running, setRunning] = useState(false);

  // One place to handle errors: a 401 means the token expired, so log out.
  function fail(err: unknown) {
    if (err instanceof ApiError && err.status === 401) return onExpired();
    setError(err instanceof Error ? err.message : "Something went wrong");
  }

  useEffect(() => {
    listSubmissions(token).then(setHistory).catch(fail);
  }, [token]);

  function changeLanguage(next: Language) {
    setLanguage(next);
    setCode(STARTER[next]);
  }

  async function run() {
    setError("");
    setRunning(true);
    try {
      const sub = await runCode(token, language, code); // can take up to ~60 s
      setResult(sub);
      setHistory((h) => [sub, ...h]);
    } catch (err) {
      fail(err);
    } finally {
      setRunning(false);
    }
  }

  return (
    <>
      <section className="card">
        <div className="row">
          <select value={language} onChange={(e) => changeLanguage(e.target.value as Language)}>
            <option value="python">Python</option>
            <option value="node">Node.js</option>
            <option value="go">Go</option>
          </select>
          <button onClick={run} disabled={running}>
            {running ? "Running..." : "Run"}
          </button>
        </div>
        <textarea value={code} onChange={(e) => setCode(e.target.value)} spellCheck={false} rows={12} />
        {running && <p className="hint">Starting a fresh container. The first run can take a few seconds (up to a minute).</p>}
        {error && <p className="error">{error}</p>}
        {result && <Output sub={result} />}
      </section>

      <section className="card">
        <h2>Past runs</h2>
        {history.length === 0 && <p className="hint">Nothing yet.</p>}
        {history.map((s) => (
          <details key={s.id}>
            <summary>
              #{s.id} · {s.language} · exit {s.exit_code}
              {s.timed_out ? " · timed out" : ""} · {new Date(s.created_at).toLocaleString()}
            </summary>
            <pre>{s.code}</pre>
            <Output sub={s} />
          </details>
        ))}
      </section>
    </>
  );
}

function Output({ sub }: { sub: Submission }) {
  return (
    <div>
      {sub.timed_out && <p className="error">Timed out (5 second limit).</p>}
      {sub.stdout && <pre className="out">{sub.stdout}</pre>}
      {sub.stderr && <pre className="out err">{sub.stderr}</pre>}
      <p className="hint">Exit code: {sub.exit_code}</p>
    </div>
  );
}
