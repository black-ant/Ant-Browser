package proxy

func (bridge *SingBoxBridge) startExitWatcher() {
	if bridge == nil || bridge.Cmd == nil {
		return
	}
	if bridge.ExitDone == nil {
		bridge.ExitDone = make(chan struct{})
	}
	bridge.waitOnce.Do(func() {
		go func() {
			err := bridge.Cmd.Wait()
			bridge.exitMu.Lock()
			bridge.ExitErr = err
			bridge.exitMu.Unlock()
			close(bridge.ExitDone)
		}()
	})
}

func (bridge *SingBoxBridge) waitExit() error {
	if bridge == nil || bridge.Cmd == nil {
		return nil
	}
	bridge.startExitWatcher()
	if bridge.ExitDone == nil {
		return nil
	}
	<-bridge.ExitDone
	return bridge.exitErr()
}

func (bridge *SingBoxBridge) exitErr() error {
	if bridge == nil {
		return nil
	}
	bridge.exitMu.Lock()
	defer bridge.exitMu.Unlock()
	return bridge.ExitErr
}
