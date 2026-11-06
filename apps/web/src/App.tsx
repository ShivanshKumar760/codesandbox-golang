import { useState } from "react";
import AuthForm from "./components/AuthForm";
import RunPanel from "./components/RunPanel";

export default function App() {
  // The token lives in memory only: a page refresh logs you out.
  // (Simple and safe from script theft; see the guide for the trade-off.)
  const [token, setToken] = useState<string | null>(null);

  return (
    <main>
      <header>
        <h1>Code Runner</h1>
        {token && <button onClick={() => setToken(null)}>Log out</button>}
      </header>
      {token ? <RunPanel token={token} onExpired={() => setToken(null)} /> : <AuthForm onLogin={setToken} />}
    </main>
  );
}
