package extract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindPath(t *testing.T) {
	type mss map[string]string
	type testPath struct{
		in    string
		out   string
		err   error
	}
	temp := os.TempDir()
	findPathTests := []struct{
		dirs  mss
		paths []testPath
	}{
		{
			// test a simple root directory mapping with various valid and invalid paths
			dirs:  mss{"/": temp},
			paths: []testPath{
				{
					in: "/test.txt",
					out: filepath.Join(temp, "test.txt"),
					err: nil,
				},{
					in: "///test.txt",
					out: filepath.Join(temp, "test.txt"),
					err: nil,
				},{
					in: "/etc/../test.txt",
					out: filepath.Join(temp, "test.txt"),
					err: nil,
				},{
					in: "test.txt",
					out: filepath.Join(temp, "test.txt"),
					err: nil,
				},{
					in: "/etc/hosts",
					out: filepath.Join(temp, "etc", "hosts"),
					err: nil,
				},{
					in: "/var/lib/rancher",
					out: filepath.Join(temp, "var", "lib", "rancher"),
					err: nil,
				},{
					in: "../../etc/passwd",
					out: "",
					err: ErrIllegalPath,
				},
			},
		},{
			// test no mapping at all
			dirs: mss{},
			paths: []testPath{
				{
					in: "/text.txt",
					out: "",
					err: nil,
				},
			},
		},{
			// test mapping various nested paths
			dirs: mss{
				"/Files/bin": filepath.Join(temp, "Files-bin"),
				"/Files": filepath.Join(temp, "Files"),
				"/etc": filepath.Join(temp, "etc"),
				},
			paths: []testPath{
				{
					in: "Files/bin",
					out: filepath.Join(temp, "Files-bin"),
					err: nil,
				},{
					in: "Files/bin/test.txt",
					out: filepath.Join(temp, "Files-bin", "test.txt"),
					err: nil,
				},{
					in: "Files/bin/aux",
					out: filepath.Join(temp, "Files-bin", "aux"),
					err: nil,
				},{
					in: "Files/bin/aux/mount",
					out: filepath.Join(temp, "Files-bin", "aux", "mount"),
					err: nil,
				},{
					in: "Files",
					out: filepath.Join(temp, "Files"),
					err: nil,
				},{
					in: "Files/test.txt",
					out: filepath.Join(temp, "Files", "test.txt"),
					err: nil,
				},{
					in: "Files/opt",
					out: filepath.Join(temp, "Files", "opt"),
					err: nil,
				},{
					in: "Files/opt/other.txt",
					out: filepath.Join(temp, "Files", "opt", "other.txt"),
					err: nil,
				},{
					in: "etc",
					out: filepath.Join(temp, "etc"),
					err: nil,
				},{
					in: "etc/hosts",
					out: filepath.Join(temp, "etc", "hosts"),
					err: nil,
				},{
					in: "etc/shadow/passwd",
					out: filepath.Join(temp, "etc", "shadow", "passwd"),
					err: nil,
				},{
					in: "sbin",
					out: "",
					err: nil,
				},{
					in: "sbin/ip",
					out: "",
					err: nil,
				},{
					in: "Files/bin/../../../../etc/passwd",
					out: "",
					err: ErrIllegalPath,
				},
			},
		},
	}

	for _, test := range findPathTests {
		t.Logf("Testing paths with dirs %#v", test.dirs)
		for _, testPath := range test.paths {
			destination, err := findPath(test.dirs, testPath.in)
			t.Logf("Got mapped path %q, err %v for image path %q", destination, err, testPath.in)
			if destination != testPath.out {
				t.Errorf("Expected path %q but got path %q for image path %q", testPath.out, destination, testPath.in)
			}
			if err != testPath.err {
				t.Errorf("Expected error %v but got error %v for image path %q", testPath.err, err, testPath.in)
			}
		}
	}
}
