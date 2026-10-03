// "Try a sample" inputs. All synthetic — never real company data.

export const samples = {
  json: '{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"api","labels":{"app":"api"}},"spec":{"replicas":3,"template":{"spec":{"containers":[{"name":"api","image":"nginx:1.27","ports":[{"containerPort":8080}]}]}}}}',

  base64Encode: 'Hello from codec — everything runs locally.',

  base64Decode: 'eyJzZXJ2aWNlIjoiYXBpIiwicmVwbGljYXMiOjMsImltYWdlIjoibmdpbng6MS4yNyJ9',

  // HS256, claims: sub, name, roles, iss, aud, iat, exp (year 2099).
  jwt: 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyLTQyIiwibmFtZSI6IkFkYSBMb3ZlbGFjZSIsInJvbGVzIjpbImRldiIsIm9wcyJdLCJpc3MiOiJodHRwczovL2F1dGguZXhhbXBsZS5jb20iLCJhdWQiOiJjb2RlYy1kZW1vIiwiaWF0IjoxNzkwMzgwODAwLCJleHAiOjQwNzA5MDg4MDB9.c2lnbmF0dXJlLW5vdC12ZXJpZmllZA',

  ansibleLog: `2026-01-05T10:00:00.0000000Z ##[group]Run ansible-playbook -i inventory.yml site.yml -v
2026-01-05T10:00:00.2000000Z ansible-playbook -i inventory.yml site.yml -v
2026-01-05T10:00:00.3000000Z ##[endgroup]
2026-01-05T10:00:01.0000000Z PLAYBOOK: site.yml *************************************************************
2026-01-05T10:00:01.4000000Z PLAY [Configure web servers] ***************************************************
2026-01-05T10:00:01.6000000Z TASK [Gathering Facts] *********************************************************
2026-01-05T10:00:03.0000000Z ok: [web-1]
2026-01-05T10:00:03.1000000Z ok: [web-2]
2026-01-05T10:00:03.2000000Z TASK [Update the package cache] ************************************************
2026-01-05T10:00:03.2100000Z task path: /builds/example/shop/ansible/site.yml:8
2026-01-05T10:00:09.0000000Z ok: [web-1] => {"cache_updated": false, "changed": false}
2026-01-05T10:00:09.4000000Z ok: [web-2] => {"cache_updated": false, "changed": false}
2026-01-05T10:00:09.5000000Z TASK [web : Install nginx] *****************************************************
2026-01-05T10:00:09.5100000Z task path: /builds/example/shop/ansible/roles/web/tasks/main.yml:1
2026-01-05T10:00:15.0000000Z changed: [web-1] => {"changed": true, "msg": "1 package installed"}
2026-01-05T10:00:15.6000000Z changed: [web-2] => {"changed": true, "msg": "1 package installed"}
2026-01-05T10:00:15.7000000Z TASK [web : Start nginx] *******************************************************
2026-01-05T10:00:15.7100000Z task path: /builds/example/shop/ansible/roles/web/tasks/main.yml:6
2026-01-05T10:00:17.0000000Z ok: [web-1] => {"changed": false, "name": "nginx", "state": "started"}
2026-01-05T10:00:17.2000000Z ok: [web-2] => {"changed": false, "name": "nginx", "state": "started"}
2026-01-05T10:00:17.3000000Z TASK [Deploy shop 0.3.0] *******************************************************
2026-01-05T10:00:17.3100000Z task path: /builds/example/shop/ansible/site.yml:17
2026-01-05T10:00:18.0000000Z changed: [web-1] => {"changed": true, "dest": "/opt/shop/shop-0.3.0.tar.gz"}
2026-01-05T10:00:18.4000000Z fatal: [web-2]: FAILED! => {"changed": false, "msg": "Could not find or access shop-0.3.0.tar.gz", "stderr": "No space left on device"}
2026-01-05T10:00:18.5000000Z RUNNING HANDLER [web : Restart web] ********************************************
2026-01-05T10:00:20.0000000Z changed: [web-1]
2026-01-05T10:00:20.1000000Z PLAY RECAP *********************************************************************
2026-01-05T10:00:20.2000000Z web-1                      : ok=4    changed=3    unreachable=0    failed=0    skipped=0    rescued=0    ignored=0
2026-01-05T10:00:20.3000000Z web-2                      : ok=3    changed=1    unreachable=0    failed=1    skipped=0    rescued=0    ignored=0
2026-01-05T10:00:21.0000000Z ##[error]Process completed with exit code 2.`,

  ansible: `TASK [installer : Run application installer] **********************************
task path: /builds/example/ansible/roles/installer/tasks/install.yml:42
fatal: [app-server-01]: FAILED! => {"changed": false, "cmd": "# Running the installer\\nbash ./installer.sh --timeout 30m --set action=\\"install\\" --set env=staging\\n", "delta": "0:00:13.244235", "end": "2026-08-18 09:35:57.276976", "msg": "non-zero return code", "rc": 1, "start": "2026-08-18 09:35:44.032741", "stderr": "[INSTALLER] - INFO: Installer action: install\\n[INSTALLER] - SEVERE: failed to fetch artifact \\"app-bundle-2.4.1\\": not found\\n[INSTALLER] - INFO: Installer finished with SEVERE errors", "stderr_lines": ["[INSTALLER] - INFO: Installer action: install", "[INSTALLER] - SEVERE: failed to fetch artifact \\"app-bundle-2.4.1\\": not found", "[INSTALLER] - INFO: Installer finished with SEVERE errors"], "stdout": "", "stdout_lines": []}`,
}
