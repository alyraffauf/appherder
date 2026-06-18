{
  buildGoModule,
}: let
  version = "dev";
in
  buildGoModule {
    pname = "appherder";
    inherit version;
    src = ./.;
    vendorHash = "sha256-hjyL3/01LWN2VTEBMwJmUyTi+tfh4zMVPWiNTkzCWfk=";
    subPackages = ["cmd/appherder"];
    ldflags = ["-X main.version=${version}"];
  }
