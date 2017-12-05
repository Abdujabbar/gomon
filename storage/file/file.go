package file

import (
	"os"
	"time"

	"github.com/iahmedov/gomon"
)

type PluginConfig struct {
}

type wrappedFile struct {
	parent              *os.File
	readSize, writeSize uint64
	readTime, writeTime time.Duration
	seekTime            time.Duration
	et                  gomon.EventTracker
}

func Wrap(f *os.File) *wrappedFile {
	et := gomon.FromContext(nil).NewChild(false)
	// in order to link other events to this event, we need to submit it to listener
	defer et.Finish()
	et.SetFingerprint("file")
	et.Set("name", f.Name())
	return &wrappedFile{
		parent: f,
		et:     et,
	}
}

func (f *wrappedFile) Chdir() (err error) {
	defer func() {
		if err != nil {
			et := f.et.NewChild(false)
			et.SetFingerprint("chdir")
			et.AddError(err)
			et.Finish()
		}
	}()
	err = f.parent.Chdir()
	return
}

func (f *wrappedFile) Chmod(mode os.FileMode) (err error) {
	et := f.et.NewChild(false)
	et.SetFingerprint("chmod")
	et.Set("mode", mode)
	defer func() {
		if err != nil {
			et.AddError(err)
		}
	}()
	defer et.Finish()
	err = f.parent.Chmod(mode)
	return
}

func (f *wrappedFile) Chown(uid, gid int) (err error) {
	et := f.et.NewChild(false)
	et.SetFingerprint("chown")
	et.Set("uid", uid)
	et.Set("gid", gid)
	defer func() {
		if err != nil {
			et.AddError(err)
		}
	}()
	defer et.Finish()
	err = f.parent.Chown(uid, gid)
	return
}

func (f *wrappedFile) Close() (err error) {
	err = f.parent.Close()
	if err != nil {
		f.et.AddError(err)
	}
	f.et.Set("read-size", f.readSize)
	f.et.Set("read-time", f.readTime)
	f.et.Set("write-size", f.writeSize)
	f.et.Set("write-time", f.writeTime)
	f.et.Set("seek-time", f.seekTime)
	f.et.Finish()
	return
}

func (f *wrappedFile) Fd() uintptr {
	return f.parent.Fd()
}

func (f *wrappedFile) Name() string {
	return f.parent.Name()
}

func (f *wrappedFile) Read(b []byte) (n int, err error) {
	start := time.Now()
	n, err = f.parent.Read(b)
	f.readSize += uint64(n)
	if err != nil {
		f.et.AddError(err)
	}
	f.readTime += time.Since(start)
	return
}

func (f *wrappedFile) ReadAt(b []byte, off int64) (n int, err error) {
	start := time.Now()
	n, err = f.parent.ReadAt(b, off)
	f.readSize += uint64(n)
	if err != nil {
		f.et.AddError(err)
	}
	f.readTime += time.Since(start)
	return
}

func (f *wrappedFile) Readdir(n int) ([]os.FileInfo, error) {
	return f.parent.Readdir(n)
}

func (f *wrappedFile) Readdirnames(n int) (names []string, err error) {
	return f.parent.Readdirnames(n)
}

func (f *wrappedFile) Seek(offset int64, whence int) (ret int64, err error) {
	start := time.Now()
	ret, err = f.parent.Seek(offset, whence)
	f.seekTime += time.Since(start)
	if err != nil {
		f.et.AddError(err)
	}
	return
}

func (f *wrappedFile) Stat() (os.FileInfo, error) {
	return f.parent.Stat()
}

func (f *wrappedFile) Sync() (err error) {
	start := time.Now()
	err = f.parent.Sync()
	f.writeTime += time.Since(start)
	if err != nil {
		f.et.AddError(err)
	}
	return err
}

func (f *wrappedFile) Truncate(size int64) (err error) {
	start := time.Now()
	err = f.parent.Truncate(size)
	f.writeTime += time.Since(start)
	if err != nil {
		f.et.AddError(err)
	}
	return
}

func (f *wrappedFile) Write(b []byte) (n int, err error) {
	start := time.Now()
	n, err = f.parent.Write(b)
	f.writeTime += time.Since(start)
	f.writeSize += uint64(n)
	if err != nil {
		f.et.AddError(err)
	}
	return
}

func (f *wrappedFile) WriteAt(b []byte, off int64) (n int, err error) {
	start := time.Now()
	n, err = f.parent.WriteAt(b, off)
	f.writeTime += time.Since(start)
	f.writeSize += uint64(n)
	if err != nil {
		f.et.AddError(err)
	}
	return
}

func (f *wrappedFile) WriteString(s string) (n int, err error) {
	return f.Write([]byte(s))
}
