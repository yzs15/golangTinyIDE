package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/docker/docker/pkg/reexec"
)

func init() {
	log.Printf("init start, os.Args = %+v\n", os.Args)
	reexec.Register("childProcess", childProcess)
	if reexec.Init() {
		os.Exit(0)
	}
}

func compileGo(srcPath string, dirPath string) (execPath string, compileOutput string, err error) {

	execPath = path.Join(dirPath, "main")
	cmd := exec.Command("go", "build", "-o", execPath, srcPath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}

	compileOutput = ""
	if err = cmd.Start(); err != nil {
		fmt.Println(2, err)
		return
	}

	// 从管道中实时获取输出并打印到终端
	for {
		tmp := make([]byte, 1024)
		_, err := stdout.Read(tmp)
		compileOutput = compileOutput + string(tmp)
		if err != nil {
			break
		}
	}

	err = cmd.Wait()
	return

}

func getUserInfo(userName string) (userinfo map[string]uint64, err error) {
	userinfo = make(map[string]uint64)
	//进程执行用户
	user, err := user.Lookup("code")
	if err != nil {
		return
	}
	uid, err := strconv.ParseUint(user.Uid, 10, 32)
	if err != nil {
		return
	}
	// 获取用户组 id
	gid, err := strconv.ParseUint(user.Gid, 10, 32)
	if err != nil {
		return
	}
	userinfo["uid"] = uid
	userinfo["gid"] = gid
	return
}

func childProcess() {
	log.Println("childProcess")
	dirPath := os.Args[1]
	srcPath := os.Args[2]

	cpu := syscall.Rlimit{Cur: 10, Max: 10}
	if err := syscall.Setrlimit(syscall.RLIMIT_CPU, &cpu); err != nil {
		os.Exit(1)
	}

	fsize := syscall.Rlimit{Cur: 20 * 1024 * 1024, Max: 20 * 1024 * 1024}
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &fsize); err != nil {
		os.Exit(1)
	}

	max_memory := syscall.Rlimit{Cur: 3 * 1024 * 1024 * 1024, Max: 3 * 1024 * 1024 * 1024}
	if err := syscall.Setrlimit(syscall.RLIMIT_AS, &max_memory); err != nil {
		os.Exit(1)
	}

	compileUserInfo, err := getUserInfo("compiler")
	if err != nil {
		os.Exit(1)
	}

	if err := os.Chown(srcPath, int(compileUserInfo["uid"]), 0); err != nil {
		os.Exit(1)
	}
	if err := os.Chmod(srcPath, 0400); err != nil {
		os.Exit(1)
	}

	execPath, compileOutput, err := compileGo(srcPath, dirPath)
	if err != nil {
		fmt.Print(compileOutput)
		os.Exit(1)
	}

	codeUserInfo, err := getUserInfo("code")
	if err != nil {
		os.Exit(1)
	}

	if err := os.Chown(execPath, int(codeUserInfo["uid"]), 0); err != nil {
		os.Exit(1)
	}
	if err := os.Chmod(execPath, 0500); err != nil {
		os.Exit(1)
	}

	attr := &syscall.SysProcAttr{
		Setpgid: true,
	}

	attr.Credential = &syscall.Credential{
		Uid:         uint32(codeUserInfo["uid"]),
		Gid:         uint32(codeUserInfo["gid"]),
		NoSetGroups: true,
	}

	cmd := exec.Command(execPath)
	// cmd.SysProcAttr.Credential = &a
	cmd.SysProcAttr = attr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		os.Exit(1)
	}

	inputReader := bufio.NewReader(os.Stdin) // 使用了自动类型推导，不需要var关键字声明

	var swg sync.WaitGroup
	swg.Add(1)
	go func() {

		defer swg.Done()
		for {
			var tmp string
			tmp, err := inputReader.ReadString('\n')

			if err != nil {
				fmt.Print(4, err)
				stdin.Close()
				break
			}
			_, err = stdin.Write([]byte(tmp))
			if err != nil {
				os.Exit(1)
			}
		}

	}()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		os.Exit(1)
	}
	err = cmd.Start()
	if err != nil {
		fmt.Println(2, err)
		os.Exit(1)
	}
	for {
		tmp := make([]byte, 1024)
		_, err := stdout.Read(tmp)
		fmt.Print(string(tmp))
		if err != nil {
			break
		}
	}
	swg.Wait()
	err = cmd.Wait()
	if err != nil {
		fmt.Print("childProcess error", err)
		os.Exit(1)
	}

}

func run(src string, dir string) (err error) {

	srcPath := path.Join(dir, "test.go")
	f, err := os.OpenFile(srcPath, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0777)
	if err != nil {
		fmt.Printf("@os.Open err=%v\n", err)
		return
	}
	n, err := f.WriteString(src)

	if n != len(src) || err != nil {
		fmt.Printf("@WriteString n=%d err=%v\n", n, err)
		return
	}

	cmd := reexec.Command("childProcess", dir, srcPath)
	// cmd.SysProcAttr.Chroot = dir
	stdin, err := cmd.StdinPipe()

	if err != nil {
		log.Println("StdinPipe exit")
		return
	}

	stdout, _ := cmd.StdoutPipe()

	// _, err = stdin.Write([]byte("1 2\n"))
	// if err != nil {
	// 	return
	// }
	// _, err = stdin.Write([]byte("3 4\n"))
	// if err != nil {
	// 	return
	// }
	// stdin.Close()
	if err = cmd.Start(); err != nil {
		log.Println("start exit")
		return
	}

	go func() {
		for {
			inputReader := bufio.NewReader(os.Stdin) // 使用了自动类型推导，不需要var关键字声明
			tmp, err := inputReader.ReadString('\n')
			if err != nil {
				return
			}
			stdin.Write([]byte(tmp + "\n"))
			// stdin.Close()
		}

	}()

	for {
		tmp := make([]byte, 1024)
		_, err := stdout.Read(tmp)
		fmt.Print(string(tmp))
		if err != nil {
			log.Println("stdout", err)
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		log.Panicf("failed to wait command: %s", err)
	}
	log.Println("main exit")
	return

}

// 判断文件夹是否存在
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func main() {
	log.Printf("main start, os.Args = %+v\n", os.Args)

	parentDir := "/home/yuzishu/CSIntroduction/goRun"
	if err := os.MkdirAll(parentDir, os.ModePerm); err != nil {
		return
	}

	var uniqID uint64

	src :=
		`package main

		import "fmt"

		func main() {
			var a, b int
			fmt.Scanf("%d %d", &a, &b)
			fmt.Println("hello", a+b)
			fmt.Scanf("%d %d", &a, &b)
			fmt.Println("hello", a+b)
			// for {
			// 	fmt.Print("1")	
			// }
		}`

	out := atomic.AddUint64(&uniqID, 1)

	dir := path.Join(parentDir, strconv.FormatUint(out, 10))
	exist, err := PathExists(dir)
	if err != nil {
		return
	}
	if exist {
		os.RemoveAll(dir)
	}
	if err = os.MkdirAll(dir, os.ModePerm); err != nil {
		return
	}

	// fmt.Println(exist)
	err = run(src, dir)
	fmt.Println(err)
	os.RemoveAll(dir)
}
