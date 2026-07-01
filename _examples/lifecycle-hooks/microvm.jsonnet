local must_env = std.native('must_env');
local caller_identity = std.native('caller_identity');

{
  Name: 'lifecycle-hooks-example',
  BaseImageArn: 'arn:aws:lambda:ap-northeast-1:aws:microvm-image:al2023-1',
  BuildRoleArn: 'arn:aws:iam::' + caller_identity().Account + ':role/LambdaMicroVMBuildRole',
  CodeArtifact: {
    uri: 's3://' + must_env('S3_BUCKET') + '/lifecycle-hooks-example/app.zip',
  },
  Hooks: {
    Port: 9000,
    MicrovmImageHooks: {
      Ready: 'ENABLED',
      ReadyTimeoutInSeconds: 60,
      Validate: 'ENABLED',
      ValidateTimeoutInSeconds: 60,
    },
  },
}
