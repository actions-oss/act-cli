package gh

import (
    "bufio"
    "bytes"
    "os/exec"
    "strings"
    "context"
    "fmt"
)

func GetToken(workingDirectory string, ctx context.Context) (string, error) {
    var token string

    // Locate the 'gh' executable
    path, err := exec.LookPath("gh")
    if err != nil {
        return "", err
    }

    // Command setup
    cmd := exec.CommandContext(ctx, path, "auth", "token")
    cmd.Dir = workingDirectory

    // Capture the output
    var out bytes.Buffer
    cmd.Stdout = &out

    // Run the command
    err = cmd.Run()
    if err != nil {
        return "", err
    }

    // Read the first line of the output
    scanner := bufio.NewScanner(&out)
    if scanner.Scan() {
        token = scanner.Text()
    }

    return token, nil
}
