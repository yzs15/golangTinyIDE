package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
)

func main() {
	src :=
		`package main
import "fmt"
	
func main() {
	fmt.Println("hello")
}`
	srcPath := path.Join("/home/yuzishu/CSIntroduction/golangTinyIDE", "test.go")
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

	cmd := exec.Command("go", "build", "-o", "main", srcPath)
	stdout, err := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout

	compileOutput := "log:"
	if err = cmd.Start(); err != nil {
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
	if err != nil {
		fmt.Print(compileOutput, err)
	}

	cmd = exec.Command("/home/yuzishu/CSIntroduction/golangTinyIDE/main")
	data, err := cmd.Output()
	fmt.Println(string(data), err)
	// cmd.Run()

}
