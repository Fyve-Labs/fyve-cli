apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTkRnNE56QTBOak13SGhjTk1qVXdOakF5TVRNeU1UQXpXaGNOTXpVd05UTXhNVE15TVRBegpXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTkRnNE56QTBOak13V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFSVGZ2Wjl0bTVCeXBVMU9Yc0NFUTlHYXdMWUxVUUNHYmpWQU9MTHpvUEoKRUtnOU80aHc3Vlk0N2ZsSTZjRTFVWXhJK1Z0eHl0a1Ruei9NVFpZZWc4K0lvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVUg0elovUXk2SG9YcURBTDNQeUpuCm1VWW82ZW93Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUloQVBqOTR5UEdYOFozLzJlK3ZvMWtndGJHeGlDdlFWYzEKUFZnVjFReHEvakFXQWlCTVpNL0ErK0wrVnJQQ1dpZ2JCQlF2M1V0UGdETmpNdis4UUtFWGN0cmRvUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    server: https://10.100.27.108:6443
  name: fyve-cli
contexts:
- context:
    cluster: fyve-cli
    user: oidc
  name: fyve-cli
current-context: fyve-cli
kind: Config
preferences: {}
users:
- name: oidc
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1
      args:
      - oidc-login
      - get-token
      - --oidc-issuer-url=https://dex.fyve.dev
      - --oidc-client-id=fyve-cluster
      - --oidc-client-secret=public
      - --oidc-extra-scope=email
      - --oidc-extra-scope=groups
      - --oidc-auth-request-extra-params=connector_id=fyve-google
      command: kubectl
      env: null
      interactiveMode: Never
      provideClusterInfo: false
