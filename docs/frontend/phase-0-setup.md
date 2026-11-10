# Frontend Phase 0 — React + Vite + TypeScript + Tailwind Setup

**Goal:** a running React app with a clean, feature-based folder structure.
**Branch:** `phase/f0-frontend-setup` · **Tag:** `f0`

## Concepts (React for a backend developer)

| React idea | Backend analogy |
|------------|-----------------|
| **Component**: a function returning JSX (HTML-like syntax) | A template function |
| **Props**: inputs passed from parent to child | Function parameters |
| **State** (`useState`): data that, when changed, re-renders the component | A variable the framework watches |
| **Effect** (`useEffect`): run code *after* render (sync with something outside React) | A side-effect hook |
| **Re-render**: React calls your function again and updates only what changed in the DOM | Re-running a pure function |
| **Server state** (TanStack Query): a cache of API data | A read-through cache |

**Golden rule:** UI = f(state). You never edit the DOM by hand. Change state, and React redraws.

**Vite** is the dev server + bundler: instant start, hot reload, and `npm run build`
outputs static files (HTML/JS/CSS) that Nginx can serve.

## Step 1 — Scaffold

```bash
cd ~/Documents/Go_ToDo_Project
git checkout -b phase/f0-frontend-setup
rmdir frontend 2>/dev/null       # remove the empty folder from B0
npm create vite@latest frontend -- --template react-ts
# If asked extra questions (experimental bundler, "install and start now"), answer No.
cd frontend
npm install
npm run dev                       # open http://localhost:5173
```

## Step 2 — Install libraries

```bash
npm install react-router @tanstack/react-query axios
npm install -D tailwindcss @tailwindcss/vite prettier @tanstack/react-query-devtools
```

| Package | Purpose |
|---------|---------|
| `react-router` | URL → page routing (`/login`, `/`) |
| `@tanstack/react-query` | Fetch, cache and sync API data |
| `axios` | HTTP client with interceptors (auto-attach JWT) |
| `tailwindcss` + `@tailwindcss/vite` | Utility-class CSS |
| `prettier` | Code formatter |
| `@tanstack/react-query-devtools` | See the query cache in the browser |

## Step 3 — Configure Vite: `vite.config.ts`

```ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    // Dev proxy: the browser calls /api/... on :5173, Vite forwards it to the backend.
    // Same origin → no CORS problems in development.
    proxy: {
      '/api': { target: 'http://localhost:8082', changeOrigin: true }, // todo-service for now; gateway (:8080) from F3
    },
  },
});
```

## Step 4 — Tailwind: replace `src/index.css`

```css
@import "tailwindcss";

body {
  @apply bg-slate-50 text-slate-900 antialiased;
}
```

Delete `src/App.css` and the Vite demo assets in `src/assets/`.

## Step 5 — Folder structure

```bash
mkdir -p src/{app,lib,hooks,pages,test} \
         src/components/{ui,layout} \
         src/features/{todos/components,auth/components,activity}
```

```
src/
├── main.tsx            # entry point
├── App.tsx             # routes
├── index.css
├── app/                # app-wide wiring (providers)
├── lib/                # framework-agnostic helpers (api client, storage)
├── hooks/              # generic reusable hooks (useDebounce)
├── components/
│   ├── ui/             # dumb building blocks: Button, Input, Spinner
│   └── layout/         # page chrome: header, main container
├── features/           # one folder per business feature
│   ├── todos/          # types, api, hooks, components for todos
│   ├── auth/
│   └── activity/
├── pages/              # one component per route; composes features
└── test/               # test setup and mocks
```

**Why feature folders?** When you work on "todos", everything is in one place.
Compare with grouping by type (`components/`, `hooks/`, `services/` for
the whole app), which scatters one feature across the tree.

## Step 6 — Minimal `src/App.tsx` and `src/main.tsx`

```tsx
// src/App.tsx
export default function App() {
  return (
    <main className="mx-auto max-w-2xl p-6">
      <h1 className="text-3xl font-bold">Go-ToDo</h1>
      <p className="mt-2 text-slate-600">Frontend is alive. Tailwind works if this is grey.</p>
    </main>
  );
}
```

```tsx
// src/main.tsx
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './index.css';
import App from './App';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
```

`StrictMode` runs some code twice in development to catch bugs. Don't be
surprised if you see effects run twice locally.

## Step 7 — Prettier & scripts

`.prettierrc`:

```json
{ "singleQuote": true, "semi": true, "printWidth": 100, "trailingComma": "all" }
```

Add to `package.json` → `"scripts"`:

```json
"format": "prettier --write src",
"typecheck": "tsc -b --noEmit"
```

`.env.example` (frontend):

```bash
# Base URL of the API. Default '/api/v1' works with the Vite proxy and with nginx.
VITE_API_URL=/api/v1
```

Only variables starting with `VITE_` are exposed to browser code, and they are
**public**: never put secrets in them.

## TypeScript notes for this template

- The template enables `verbatimModuleSyntax`: import **types** with `import type { Todo } from ...`.
- It enables `erasableSyntaxOnly`: use **union types** (`'low' | 'medium'`) instead of `enum`.

## Verify

```bash
npm run dev        # page shows, grey text
npm run lint       # no errors
npm run typecheck
npm run build      # creates dist/
```

## Commit

```bash
cd ..
git add -A
git commit -m "chore(frontend): scaffold Vite React TypeScript app with Tailwind (F0)"
```

**Next:** [Phase 1 — UI with local state](phase-1-ui-with-local-state.md)
