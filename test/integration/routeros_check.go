//go:build ignore

package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/go-routeros/routeros/v3"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// First try plain TCP to see raw protocol response
	fmt.Println("1. Testing raw TCP read...")
	conn, err := net.DialTimeout("tcp", "localhost:8728", 5*time.Second)
	if err != nil {
		fmt.Printf("  TCP error: %v\n", err)
	} else {
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		buf := make([]byte, 256)
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Printf("  Raw read: err=%v (this is normal - API waits for client)\n", err)
		} else {
			fmt.Printf("  Raw read: %d bytes: %x\n", n, buf[:n])
		}
		conn.Close()
	}

	// Try go-routeros with empty password (RouterOS 7 default)
	fmt.Println("\n2. Trying DialContext with password=\"\"...")
	c, err := routeros.DialContext(ctx, "localhost:8728", "admin", "")
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Println("  SUCCESS with empty password!")
		reply, _ := c.RunContext(ctx, "/system/resource/print")
		if reply != nil {
			for _, re := range reply.Re {
				fmt.Printf("  Platform: %s, Version: %s\n", re.Map["platform"], re.Map["version"])
			}
		}
		c.Close()
		return
	}

	// Try go-routeros with "admin" password
	fmt.Println("\n3. Trying DialContext with password=\"admin\"...")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	c2, err := routeros.DialContext(ctx2, "localhost:8728", "admin", "admin")
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Println("  SUCCESS with admin password!")
		reply, _ := c2.RunContext(ctx2, "/system/resource/print")
		if reply != nil {
			for _, re := range reply.Re {
				fmt.Printf("  Platform: %s, Version: %s\n", re.Map["platform"], re.Map["version"])
			}
		}
		c2.Close()
		return
	}

	// Try TLS on 8729
	fmt.Println("\n4. Trying DialTLS on 8729...")
	ctx3, cancel3 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel3()
	c3, err := routeros.DialTLSContext(ctx3, "localhost:8729", "admin", "", nil)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Println("  SUCCESS with TLS!")
		c3.Close()
	}
}
