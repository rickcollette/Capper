package api

import (
	"net/http"
	"sync"

	"capper/internal/types"

	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		return origin == "http://"+r.Host || origin == "https://"+r.Host
	},
}

func (s *Server) handleInstanceTerminal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.authorize(r, "instance:connect", "instance/"+id); err != nil {
		writeForbidden(w, err)
		return
	}
	inst, err := s.ctrl.Store.ResolveInstance(id)
	if err != nil {
		writeNotFound(w, "instance not found")
		return
	}
	if inst.Status != types.StatusRunning {
		writeBadRequest(w, errTerminalNotRunning)
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	cmd, ptyFile, err := s.ctrl.Instances.StartShellPTY(inst.Name)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("terminal start failed: "+err.Error()))
		return
	}
	defer ptyFile.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, readErr := ptyFile.Read(buf)
			if n > 0 {
				_ = conn.WriteMessage(websocket.TextMessage, buf[:n])
			}
			if readErr != nil {
				return
			}
		}
	}()

	for {
		_, msg, readErr := conn.ReadMessage()
		if readErr != nil {
			break
		}
		_, _ = ptyFile.Write(msg)
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	wg.Wait()
}

var errTerminalNotRunning = terminalError{"instance is not running"}

type terminalError struct{ msg string }

func (e terminalError) Error() string { return e.msg }
