# onnx.nix - Overlay for ONNX Runtime 1.23.0 using prebuilt binaries
final: prev: {
  onnxruntime = let
    version = "1.23.0";
    
    # Determine the correct download URL and hash based on system architecture
    srcInfo = {
      x86_64-linux = {
        url = "https://github.com/microsoft/onnxruntime/releases/download/v${version}/onnxruntime-linux-x64-${version}.tgz";
        sha256 = "sha256-tt7qfy4iwQwEMBnylKDqTSpsCuUqAJw0hHZA23XsVYA=";
      };
    }.${prev.stdenv.hostPlatform.system} or (throw "Unsupported system: ${prev.stdenv.hostPlatform.system}");
    
  in
    prev.stdenv.mkDerivation rec {
      pname = "onnxruntime";
      inherit version;
      
      src = prev.fetchurl {
        url = srcInfo.url;
        sha256 = srcInfo.sha256;
      };
      
      # Don't configure or build - we're using prebuilt binaries
      dontConfigure = true;
      dontBuild = true;
      
      installPhase = ''
        runHook preInstall
        
        # Extract directly to $out
        mkdir -p $out
        tar -xzf $src -C $out --strip-components=1
        
        # Set proper rpath for the library
        patchelf --set-rpath "${prev.lib.makeLibraryPath [ prev.stdenv.cc.cc.lib ]}:$out/lib" \
          $out/lib/libonnxruntime.so || true
        
        runHook postInstall
      '';
      
      # Create pkg-config file
      postInstall = ''
        mkdir -p $out/lib/pkgconfig
        cat > $out/lib/pkgconfig/onnxruntime.pc <<EOF
        prefix=$out
        exec_prefix=''${prefix}
        libdir=''${exec_prefix}/lib
        includedir=''${prefix}/include
        
        Name: ONNXRuntime
        Description: ONNX Runtime is a performance-focused inference engine for ONNX models
        Version: ${version}
        Libs: -L''${libdir} -lonnxruntime
        Cflags: -I''${includedir}
EOF
      '';
      
      meta = with prev.lib; {
        description = "Cross-platform, high performance ML inferencing accelerator (prebuilt)";
        homepage = "https://onnxruntime.ai";
        license = licenses.mit;
        platforms = [ "x86_64-linux" "aarch64-linux" ];
        maintainers = [ ];
      };
    };
}
