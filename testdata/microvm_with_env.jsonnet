{
  Name: std.native("must_env")("TEST_MICROVM_NAME"),
  BaseImageArn: "arn:aws:lambda:ap-northeast-1:aws:microvm-image:al2023-1",
  BuildRoleArn: "arn:aws:iam::" + std.native("env")("TEST_ACCOUNT_ID", "000000000000") + ":role/TestBuildRole",
  CodeArtifact: {
    Uri: "s3://test-bucket/artifact.zip",
  },
}
