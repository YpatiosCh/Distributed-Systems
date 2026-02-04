# Chapter 3: Write a Log Package

## Overview

This chapter implements a production-grade commit log with persistent storage. You'll build the foundation of the distributed log service using memory-mapped files, efficient indexing, and segment-based storage management.

## What You'll Learn

- Building an append-only log with persistent storage
- Using memory-mapped files for zero-copy I/O
- Implementing efficient index structures
- Managing log segments and automatic rotation
- Handling file I/O, seeking, and truncation
- Writing comprehensive tests for storage systems

## Why Persistent Storage?

The in-memory log from Chapter 1 has critical limitations:
- ❌ Data lost on restart
- ❌ Limited by RAM size
- ❌ No durability guarantees
- ❌ Can't replay old data

A persistent log provides:
- ✅ Durability across restarts
- ✅ Bounded memory usage
- ✅ Historical data access
- ✅ Disaster recovery

## Architecture Overview

```
Log
 ├── Segment 1 (offset 0-999)
 │   ├── store (data file)
 │   └── index (offset → position lookup)
 ├── Segment 2 (offset 1000-1999)
 │   ├── store
 │   └── index
 └── Active Segment (offset 2000+)
     ├── store
     └── index
```

## Project Structure

```
WriteALogPackage/
├── api/
│   └── v1/
│       ├── log.proto
│       └── log.pb.go
├── internal/
│   └── log/
│       ├── config.go        # Configuration structure
│       ├── index.go         # Index file implementation
│       ├── index_test.go
│       ├── log.go           # Main log coordinator
│       ├── log_test.go
│       ├── segment.go       # Segment (store + index)
│       ├── segment_test.go
│       ├── store.go         # Store file implementation
│       └── store_test.go
├── go.mod
└── go.sum
```

## Core Components

### 1. Store (`store.go`)

The store is an append-only file that holds the actual record data.

**Key Features:**
- Uses `os.OpenFile` with append mode
- Memory-mapped for fast reads with `gommap`
- Buffered writes with `bufio.Writer`
- Efficient append operation

**Operations:**
```go
type store struct {
    *os.File
    mu   sync.Mutex
    buf  *bufio.Writer
    size uint64
}

// Append writes data and returns position + bytes written
func (s *store) Append(p []byte) (n uint64, pos uint64, err error)

// Read returns len bytes starting at position pos
func (s *store) Read(pos uint64) ([]byte, error)

// ReadAt implements io.ReaderAt for streaming
func (s *store) ReadAt(p []byte, off int64) (int, error)

// Close flushes and closes the file
func (s *store) Close() error
```

**File Format:**
```
[record length: 8 bytes][record data: N bytes][record length: 8 bytes][record data: N bytes]...
```

### 2. Index (`index.go`)

The index provides fast offset-to-position lookups.

**Key Features:**
- Memory-mapped for zero-copy reads
- Fixed-size entries (12 bytes each)
- Binary format for compactness
- Bounded by max size

**Entry Format:**
```
[offset: 4 bytes][position: 8 bytes] = 12 bytes per entry
```

**Operations:**
```go
type index struct {
    file *os.File
    mmap gommap.MMap
    size uint64
}

// Write adds an entry to the index
func (i *index) Write(off uint32, pos uint64) error

// Read returns position for the given offset entry
func (i *index) Read(in int64) (out uint32, pos uint64, err error)

// Close syncs and closes the index
func (i *index) Close() error
```

**Why Memory-Mapped Files?**
- Zero-copy: OS maps file directly to memory
- Fast: No system calls for reads
- Efficient: OS handles caching automatically
- Safe: Changes synced to disk by OS

### 3. Segment (`segment.go`)

A segment combines a store and index for a range of offsets.

**Lifecycle:**
- Created when log starts or previous segment fills
- Becomes read-only when full
- Removed when truncated

**Operations:**
```go
type segment struct {
    store      *store
    index      *index
    baseOffset uint64
    nextOffset uint64
    config     Config
}

// Append adds a record to the segment
func (s *segment) Append(record *api.Record) (offset uint64, err error)

// Read retrieves a record by offset
func (s *segment) Read(off uint64) (*api.Record, error)

// IsMaxed returns true if segment is full
func (s *segment) IsMaxed() bool

// Remove deletes segment files
func (s *segment) Remove() error

// Close closes both store and index
func (s *segment) Close() error
```

**Segment Rotation:**
Segments rotate when either:
- Store size exceeds `MaxStoreBytes`
- Index size exceeds `MaxIndexBytes`

### 4. Log (`log.go`)

The log coordinates multiple segments and provides a unified API.

**Responsibilities:**
- Manage segment lifecycle
- Route reads/writes to correct segment
- Handle segment rotation
- Maintain offset ranges
- Support log truncation

**Operations:**
```go
type Log struct {
    mu            sync.RWMutex
    Dir           string
    Config        Config
    activeSegment *segment
    segments      []*segment
}

// NewLog creates or loads a log from directory
func NewLog(dir string, c Config) (*Log, error)

// Append adds a record and returns its offset
func (l *Log) Append(record *api.Record) (uint64, error)

// Read retrieves a record by offset
func (l *Log) Read(off uint64) (*api.Record, error)

// Close closes all segments
func (l *Log) Close() error

// Remove deletes the log directory
func (l *Log) Remove() error

// Reset removes and recreates the log
func (l *Log) Reset() error

// LowestOffset returns the first offset
func (l *Log) LowestOffset() (uint64, error)

// HighestOffset returns the last offset
func (l *Log) HighestOffset() (uint64, error)

// Truncate removes segments with offsets <= lowest
func (l *Log) Truncate(lowest uint64) error

// Reader returns an io.Reader for the entire log
func (l *Log) Reader() io.Reader
```

## Configuration

```go
type Config struct {
    Segment struct {
        MaxStoreBytes uint64  // Max store file size
        MaxIndexBytes uint64  // Max index file size
        InitialOffset uint64  // Starting offset
    }
}
```

**Default Values:**
```go
MaxStoreBytes: 1024        // 1KB (small for testing)
MaxIndexBytes: 1024        // 1KB
InitialOffset: 0           // Start from 0
```

**Production Values:**
```go
MaxStoreBytes: 1073741824  // 1GB
MaxIndexBytes: 10485760    // 10MB
InitialOffset: 0
```

## File Naming Convention

```
/data/log/
├── 0.store    # First segment store
├── 0.index    # First segment index
├── 1000.store # Second segment (after rotation)
├── 1000.index
├── 2000.store # Third segment
└── 2000.index
```

File names are the segment's base offset.

## Usage Example

```go
// Create a log
dir := "/tmp/mylog"
config := Config{}
config.Segment.MaxStoreBytes = 1024
config.Segment.MaxIndexBytes = 1024

log, err := NewLog(dir, config)
if err != nil {
    panic(err)
}
defer log.Close()

// Append records
record := &api.Record{Value: []byte("hello world")}
offset, err := log.Append(record)
if err != nil {
    panic(err)
}

// Read records
read, err := log.Read(offset)
if err != nil {
    panic(err)
}
fmt.Println(string(read.Value)) // "hello world"
```

## Testing Strategy

The package includes comprehensive tests:

### Unit Tests
- `store_test.go` - Store append/read operations
- `index_test.go` - Index write/read operations
- `segment_test.go` - Segment lifecycle and rotation

### Integration Tests
- `log_test.go` - Multi-segment scenarios
  - Append and read across segments
  - Segment rotation behavior
  - Log persistence across restarts
  - Truncation operations
  - Reader interface

**Run Tests:**
```bash
go test -v ./internal/log/...
```

## Performance Characteristics

| Operation | Complexity | Notes |
|-----------|------------|-------|
| Append | O(1) | Buffered writes |
| Read by offset | O(log n) | Binary search index |
| Sequential read | O(1) | Memory-mapped |
| Segment rotation | O(1) | Create new segment |
| Truncation | O(n) | Remove n segments |

## Key Design Decisions

### Why Segments?
- **Bounded files**: Easier to manage than unbounded
- **Faster operations**: Smaller files = faster I/O
- **Easy cleanup**: Delete old segments, keep recent
- **Parallel access**: Different segments can be read concurrently

### Why Memory-Mapped Files?
- **Zero-copy**: No buffer copies between kernel and user space
- **Fast reads**: Data accessed like regular memory
- **OS-managed**: Page cache handles caching automatically
- **Simple code**: No manual buffer management

### Why Fixed-Size Index Entries?
- **Binary search**: Enables O(log n) lookups
- **Predictable memory**: Know exact index size
- **Fast writes**: No variable-length encoding overhead

## Error Handling

Common errors:
- `ErrOffsetNotFound` - Offset doesn't exist
- `ErrOffsetOutOfRange` - Offset beyond highest
- `io.EOF` - Reached end of log
- File I/O errors - Disk full, permissions, corruption

## Limitations

Current implementation limitations:
- No compression
- No checksums (corruption detection)
- No encryption
- Single writer (no concurrent append)
- No replication
- No distributed coordination

These will be addressed in later chapters.

## Key Takeaways

✅ Append-only logs are simple and efficient  
✅ Memory-mapped files provide zero-copy I/O  
✅ Segmentation bounds file sizes and enables cleanup  
✅ Indexes enable fast random access  
✅ Proper testing is critical for storage systems  

## Next Steps

Move to **Chapter 4: Serve Requests** to learn how to:
- Wrap the log in a gRPC service
- Add network communication
- Support streaming operations
- Handle concurrent clients

## Dependencies

```
github.com/stretchr/testify v1.8.2     # Testing assertions
github.com/tysonmote/gommap v0.0.2     # Memory-mapped files
google.golang.org/protobuf v1.28.1     # Protocol Buffers
```

## Additional Resources

- [Memory-Mapped Files Explained](https://en.wikipedia.org/wiki/Memory-mapped_file)
- [LSM Trees and Log-Structured Storage](https://en.wikipedia.org/wiki/Log-structured_merge-tree)
- [The Log-Structured Merge-Tree (LSM-Tree)](http://www.benstopford.com/2015/02/14/log-structured-merge-trees/)

---

**Ready for networking?** The next chapter wraps your log in a gRPC service! 🌐