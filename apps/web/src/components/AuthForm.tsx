import { useState, type FormEvent } from "react";
import { login, register } from "../api";

export default function AuthForm({ onLogin }: { onLogin: (token: string) => void }) {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      if (mode === "register") await register(email, password);
      // After registering we log straight in.
      onLogin(await login(email, password));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="card">
      <h2>{mode === "login" ? "Log in" : "Create account"}</h2>
      <input type="email" placeholder="Email" value={email} onChange={(e) => setEmail(e.target.value)} required />
      <input
        type="password"
        placeholder="Password (8+ characters)"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        minLength={8}
        required
      />
      {error && <p className="error">{error}</p>}
      <button disabled={busy}>{busy ? "Please wait..." : mode === "login" ? "Log in" : "Register"}</button>
      <button type="button" className="link" onClick={() => setMode(mode === "login" ? "register" : "login")}>
        {mode === "login" ? "No account? Register" : "Have an account? Log in"}
      </button>
    </form>
  );
}
