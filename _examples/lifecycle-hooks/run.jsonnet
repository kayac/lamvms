{
  IngressNetworkConnectors: [
    'arn:aws:lambda:ap-northeast-1:aws:network-connector:aws-network-connector:HTTP_INGRESS',
  ],
  EgressNetworkConnectors: [
    'arn:aws:lambda:ap-northeast-1:aws:network-connector:aws-network-connector:INTERNET_EGRESS',
  ],
  IdlePolicy: {
    AutoResumeEnabled: true,
    MaxIdleDurationSeconds: 900,
    SuspendedDurationSeconds: 300,
  },
}
