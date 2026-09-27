// "Try a sample" inputs. All synthetic — never real company data.

export const samples = {
  json: '{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"api","labels":{"app":"api"}},"spec":{"replicas":3,"template":{"spec":{"containers":[{"name":"api","image":"nginx:1.27","ports":[{"containerPort":8080}]}]}}}}',

  base64Encode: 'Hello from codec — everything runs locally.',

  base64Decode: 'eyJzZXJ2aWNlIjoiYXBpIiwicmVwbGljYXMiOjMsImltYWdlIjoibmdpbng6MS4yNyJ9',

  // HS256, claims: sub, name, roles, iss, aud, iat, exp (year 2099).
  jwt: 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyLTQyIiwibmFtZSI6IkFkYSBMb3ZlbGFjZSIsInJvbGVzIjpbImRldiIsIm9wcyJdLCJpc3MiOiJodHRwczovL2F1dGguZXhhbXBsZS5jb20iLCJhdWQiOiJjb2RlYy1kZW1vIiwiaWF0IjoxNzkwMzgwODAwLCJleHAiOjQwNzA5MDg4MDB9.c2lnbmF0dXJlLW5vdC12ZXJpZmllZA',

  ansible: `TASK [installer : Run application installer] **********************************
task path: /builds/example/ansible/roles/installer/tasks/install.yml:42
fatal: [app-server-01]: FAILED! => {"changed": false, "cmd": "# Running the installer\\nbash ./installer.sh --timeout 30m --set action=\\"install\\" --set env=staging\\n", "delta": "0:00:13.244235", "end": "2026-08-18 09:35:57.276976", "msg": "non-zero return code", "rc": 1, "start": "2026-08-18 09:35:44.032741", "stderr": "[INSTALLER] - INFO: Installer action: install\\n[INSTALLER] - SEVERE: failed to fetch artifact \\"app-bundle-2.4.1\\": not found\\n[INSTALLER] - INFO: Installer finished with SEVERE errors", "stderr_lines": ["[INSTALLER] - INFO: Installer action: install", "[INSTALLER] - SEVERE: failed to fetch artifact \\"app-bundle-2.4.1\\": not found", "[INSTALLER] - INFO: Installer finished with SEVERE errors"], "stdout": "", "stdout_lines": []}`,
}
