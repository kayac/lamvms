{
  Name: 'test-microvm-jsonnet-braces',
  BaseImageArn: 'arn:aws:lambda:ap-northeast-1:aws:microvm-image:al2023-1',
  BuildRoleArn: 'arn:aws:iam::123456789012:role/TestBuildRole',
  Description: 'contains literal braces: {{ not a go template }}',
  CodeArtifact: {
    Uri: 's3://test-bucket/artifact.zip',
  },
}
