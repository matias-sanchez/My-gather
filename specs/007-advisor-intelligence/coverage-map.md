# Advisor Rosetta Stone Coverage Map

This feature covers the Rosetta Stone topics that can be supported by existing
pt-stalk report inputs. Topics that need new parser coverage are explicitly
deferred rather than inferred.

## Covered

- Buffer Pool: hit ratio, free-page pressure, LRU flushing, wait-free stalls,
  dirty-page pressure, and undersized pool heuristics.
- Redo Log: checkpoint age, pending writes, pending fsyncs, and log-buffer
  waits.
- Flushing and Dirty Pages: InnoDB pending write paths, single-page flushes,
  flush-list backlog, and buffer-pool fsync backlog.
- Table Open Cache: usage, misses, and overflows.
- Thread Cache: thread creation churn and cache effectiveness.
- Connections: connection saturation and aborted connection rates.
- Temporary Tables: disk temporary table ratio.
- Query Shape: full scans, full joins, random-next reads, processlist abuse,
  and observed slow active queries.
- Semaphores: InnoDB semaphore waits from SHOW ENGINE INNODB STATUS.
- Binary Log Caches: transactional and statement cache disk use.

## Partial

- Metadata and DDL Contention: covered through processlist metadata-lock waits
  and DDL-related counters when available. Prepared-statement reprepare errors
  remain partial because the report does not currently parse MySQL error logs.

## Deferred

- Change Buffer: needs additional InnoDB status parsing for change-buffer size,
  merges, and free-list evidence.
- Adaptive Hash Index: needs additional InnoDB status or INNODB_METRICS input.
- Data Dictionary internals: needs version-specific dictionary wait and
  error-log evidence before Advisor can make supported findings.

## Scope Note

The Advisor must not read `MySQL Rosetta Stone.txt` at runtime. The source file
is planning evidence only; report output must be based on parsed capture data
and checked-in coverage metadata.
