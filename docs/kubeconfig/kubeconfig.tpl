apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkakNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTkRZM09EUXpPVEF3SGhjTk1qVXdOVEE1TURrMU16RXdXaGNOTXpVd05UQTNNRGsxTXpFdwpXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTkRZM09EUXpPVEF3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFUeThtYnBIWWtMZGdEV1NTWE0vSGdhOTExbU5Oc2hSclhybGs5MmxoWmsKUXYremMwNUdmSUl3MWJhTWU4VnoxNDFhNU96bStQeXRDbVZVY0krVFpYdldvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVWt1OUZGTDcxQ0Nza2JkSmNCRUIzCm4xc1hiSUF3Q2dZSUtvWkl6ajBFQXdJRFJ3QXdSQUlnWTlqUk0vQ202RXljNXpRU1FYTnVIV3lla2Q3VURhS00KVmhHdEowZHh1RklDSUE1QkFZU0FpSFgzY2d2dGZrN0lVWVlOdmlIalBsMWRBR3RCaCtYeVZTaysKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
    server: https://10.100.29.101:6443
  name: fyve-k8s
contexts:
- context:
    cluster: fyve-k8s
    user: oidc
  name: fyve-k8s
current-context: fyve-k8s
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
      - --oidc-issuer-url=https://auth.fyve.dev
      - --oidc-client-id=fyve-k8s
      command: kubectl
      env: null
      interactiveMode: Never
      provideClusterInfo: false
