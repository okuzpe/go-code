package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// CreateProjectTool scaffolds a new project directory from a built-in template.
type CreateProjectTool struct {
	scope PathScope
}

// NewCreateProject returns create_project with Root and RelativeBase both set to root.
func NewCreateProject(root string) *CreateProjectTool {
	return NewCreateProjectScope(PathScope{Root: root, RelativeBase: root})
}

// NewCreateProjectScope returns create_project with explicit PathScope.
func NewCreateProjectScope(scope PathScope) *CreateProjectTool {
	s := NormalizePathScope(scope)
	if strings.TrimSpace(s.RelativeBase) == "" {
		s.RelativeBase = s.Root
	}
	return &CreateProjectTool{scope: s}
}

var _ Tool = (*CreateProjectTool)(nil)

func (CreateProjectTool) Name() string { return "create_project" }

func (CreateProjectTool) Description() string {
	return "Scaffold a new project directory from a built-in template. " +
		"Supported types: go, nodejs, python, electron, react, fastapi. " +
		"Creates the directory tree and boilerplate files (go.mod, main.go, package.json, main.js, etc.). " +
		"Does NOT run package managers — use bash after scaffolding to run 'go mod tidy', 'npm install', etc."
}

func (CreateProjectTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type": map[string]any{
				"type":        "string",
				"enum":        []string{"go", "nodejs", "python", "electron", "react", "fastapi"},
				"description": "Project template type.",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "Project or module name used in go.mod, package.json, etc.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory path where the project will be created (created if missing).",
			},
		},
		"required": []string{"type", "name", "path"},
	}
}

type createProjectInput struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// projectTemplates maps type → filename → content template.
// {{NAME}} is substituted with the project name at scaffold time.
var projectTemplates = map[string]map[string]string{
	"go": {
		"go.mod": "module {{NAME}}\n\ngo 1.22\n",
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello from {{NAME}}!")
}
`,
		".gitignore": "# Go\n/{{NAME}}\n*.exe\n*.exe~\n*.dll\n*.so\n*.dylib\n*.test\n*.out\nvendor/\n",
		"README.md":  "# {{NAME}}\n\nA Go project.\n\n## Build\n\n```bash\ngo build ./...\ngo test ./...\n```\n",
	},
	"nodejs": {
		"package.json": `{
  "name": "{{NAME}}",
  "version": "0.1.0",
  "description": "",
  "main": "index.js",
  "scripts": {
    "start": "node index.js",
    "test": "echo \"No tests yet\" && exit 1"
  },
  "keywords": [],
  "license": "ISC"
}
`,
		"index.js":   "\"use strict\";\n\nconsole.log(\"Hello from {{NAME}}!\");\n",
		".gitignore": "node_modules/\n.env\n*.log\n",
		"README.md":  "# {{NAME}}\n\nA Node.js project.\n\n## Run\n\n```bash\nnpm install\nnode index.js\n```\n",
	},
	"python": {
		"pyproject.toml": `[build-system]
requires = ["setuptools>=68"]
build-backend = "setuptools.backends.legacy:build"

[project]
name = "{{NAME}}"
version = "0.1.0"
requires-python = ">=3.10"
`,
		"src/{{NAME}}/__init__.py": "# {{NAME}}\n",
		"src/{{NAME}}/main.py":     "def main() -> None:\n    print(\"Hello from {{NAME}}!\")\n\n\nif __name__ == \"__main__\":\n    main()\n",
		".gitignore":               "__pycache__/\n*.py[cod]\n*.egg-info/\ndist/\nbuild/\n.venv/\n.env\n",
		"README.md":                "# {{NAME}}\n\nA Python project.\n\n## Run\n\n```bash\npython -m {{NAME}}.main\n```\n",
	},
	"electron": {
		"package.json": `{
  "name": "{{NAME}}",
  "version": "0.1.0",
  "description": "{{NAME}} desktop app",
  "main": "main.js",
  "scripts": {
    "start": "electron ."
  },
  "devDependencies": {
    "electron": "^30.0.0"
  }
}
`,
		"main.js": `const { app, BrowserWindow } = require("electron");

function createWindow() {
  const win = new BrowserWindow({
    width: 1024,
    height: 768,
    webPreferences: { nodeIntegration: true, contextIsolation: false },
  });
  win.loadFile("index.html");
}

app.whenReady().then(createWindow);
app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});
`,
		"index.html": `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <title>{{NAME}}</title>
  <link rel="stylesheet" href="style.css" />
</head>
<body>
  <h1>{{NAME}}</h1>
  <div id="app"></div>
  <script src="renderer.js"></script>
</body>
</html>
`,
		"style.css": `body { margin: 0; font-family: sans-serif; background: #1a1a2e; color: #eee; }
h1 { text-align: center; padding: 1rem; }
#app { display: flex; justify-content: center; padding: 1rem; }
`,
		"renderer.js": "// Renderer process — add your UI logic here.\nconsole.log(\"{{NAME}} renderer ready.\");\n",
		".gitignore":  "node_modules/\ndist/\n*.log\n",
		"README.md": `# {{NAME}}

Electron desktop app (web technologies).

## Run

` + "```" + `bash
npm install
npm start
` + "```" + `
`,
	},
	"react": {
		"package.json": `{
  "name": "{{NAME}}",
  "version": "0.1.0",
  "private": true,
  "dependencies": {
    "react": "^18.3.0",
    "react-dom": "^18.3.0"
  },
  "devDependencies": {
    "vite": "^5.0.0",
    "@vitejs/plugin-react": "^4.0.0"
  },
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  }
}
`,
		"vite.config.js": `import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({ plugins: [react()] });
`,
		"index.html": `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>{{NAME}}</title>
</head>
<body>
  <div id="root"></div>
  <script type="module" src="/src/main.jsx"></script>
</body>
</html>
`,
		"src/main.jsx": `import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App.jsx";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root")).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
`,
		"src/App.jsx": `import React from "react";

export default function App() {
  return (
    <div style={{ padding: "2rem", fontFamily: "sans-serif" }}>
      <h1>{{NAME}}</h1>
      <p>Edit <code>src/App.jsx</code> to get started.</p>
    </div>
  );
}
`,
		"src/index.css": `*, *::before, *::after { box-sizing: border-box; }
body { margin: 0; font-family: system-ui, sans-serif; }
`,
		".gitignore": "node_modules/\ndist/\n.env\n*.log\n",
		"README.md": `# {{NAME}}

React app powered by Vite.

## Run

` + "```" + `bash
npm install
npm run dev
` + "```" + `
`,
	},
	"fastapi": {
		"pyproject.toml": `[build-system]
requires = ["setuptools>=68"]
build-backend = "setuptools.backends.legacy:build"

[project]
name = "{{NAME}}"
version = "0.1.0"
requires-python = ">=3.10"
dependencies = ["fastapi>=0.111", "uvicorn[standard]>=0.29"]
`,
		"src/{{NAME}}/main.py": `from fastapi import FastAPI

app = FastAPI(title="{{NAME}}")


@app.get("/")
def root() -> dict:
    return {"message": "Hello from {{NAME}}!"}


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}
`,
		"src/{{NAME}}/__init__.py": "# {{NAME}}\n",
		".gitignore":               "__pycache__/\n*.py[cod]\n*.egg-info/\ndist/\nbuild/\n.venv/\n.env\n",
		"README.md": `# {{NAME}}

FastAPI application.

## Run

` + "```" + `bash
pip install -e .
uvicorn {{NAME}}.main:app --reload
` + "```" + `
`,
	},
}

// Execute implements Tool.
func (t *CreateProjectTool) Execute(_ context.Context, input string) (Result, error) {
	var in createProjectInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: fmt.Sprintf("invalid json input: %v", err), IsError: true}, nil
	}

	in.Type = strings.TrimSpace(in.Type)
	in.Name = strings.TrimSpace(in.Name)
	in.Path = strings.TrimSpace(in.Path)

	if in.Type == "" {
		return Result{Content: "type is required (go, nodejs, python)", IsError: true}, nil
	}
	if in.Name == "" {
		return Result{Content: "name is required", IsError: true}, nil
	}
	if in.Path == "" {
		return Result{Content: "path is required", IsError: true}, nil
	}

	tmpl, ok := projectTemplates[in.Type]
	if !ok {
		return Result{Content: fmt.Sprintf("unsupported project type %q; valid types: go, nodejs, python, electron, react, fastapi", in.Type), IsError: true}, nil
	}

	results := make([]string, 0, len(tmpl))
	anyError := false

	for relPath, content := range tmpl {
		// Substitute {{NAME}} in both the path and the content.
		relPath = strings.ReplaceAll(relPath, "{{NAME}}", in.Name)
		content = strings.ReplaceAll(content, "{{NAME}}", in.Name)

		fullPath := in.Path + "/" + relPath
		resolved, err := ResolveWriteTargetPath(t.scope, fullPath)
		if err != nil {
			results = append(results, fmt.Sprintf("ERROR %s: %v", fullPath, err))
			anyError = true
			continue
		}
		if err := atomicWriteFile(resolved, []byte(content), 0o600); err != nil {
			results = append(results, fmt.Sprintf("ERROR %s: %v", fullPath, err))
			anyError = true
			continue
		}
		results = append(results, fmt.Sprintf("wrote %s", fullPath))
	}

	return Result{Content: strings.Join(results, "\n"), IsError: anyError}, nil
}
