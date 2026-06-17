{
  buildGoModule,
  lib,
}:
buildGoModule {
  pname = "appherder";
  version = "dev";
  src = ./.;
  vendorHash = "sha256-Q+emMKLlnoRlYIe2nNZ6NKkg6bao1xj8CARkv5uiZRs=";
  subPackages = ["cmd/appherder"];
}
