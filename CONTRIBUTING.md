Logging
-------

- All output should use loggers (no bare writes to stdout or stderr).

- Except for the Main function, the logger should always be obtained from
  the current `context.Context`. (If you want to log something and your
  function doesn't have a `Context`, you should add it as the first argument.)

- Logs should use fields for all arguments, with the log message itself being
  a fixed string.  For example:

      // BAD - don't do this!
      logger.Info("Committing " + publish.ID())

      // Good!
      logger.F("publish", publish.ID()).Info("Committing")

- Logs at "info" level should generally be a terse summary of what has happened,
  or what is about to happen, with a leading capital letter but no period as it
  is not a full sentence. For example:

      // BAD - don't do this!
      logger.F("task", task.ID()).Info("the publish task has finished.")

      // Good!
      logger.F("task", task.ID()).Info("Task completed")

- Logs of level "info" and higher will be seen by default, and should use
  human-readable fields. The default string formatting of Go structs, maps
  and arrays is not considered to be human-readable.

- Logs of level "debug" will not be seen by default and need not use only
  human-readable fields.

- Usage of defer/Trace/Stop is encouraged if it doesn't complicate the code.
  Note that it typically requires hoisting `error` to the top of the function.


Goroutines
----------

- If you spawn any goroutine, you must accept a `context.Context` and ensure
  your goroutine returns promptly if the context becomes Done.

- It is recommended not to expose the existence of goroutines across package
  boundaries. For example, public functions in a package shouldn't use channels
  to communicate their outputs.


Error handling
--------------

- Errors should almost always be propagated to the caller.

- Errors should usually be wrapped while propagating to add some brief context.
  `fmt.Errorf` with the `%w` directive can be used for this. For example:

      // While loading content from some file...
      err = dec.Decode(&out)
      if err != nil {

        // BAD - don't do this!
        // Caller will get a bare parser error with no way of telling which file
        // couldn't be parse.
        return out, err

        // Good!
        // Caller will know which file couldn't be parsed.
        return out, fmt.Errorf("parsing %s: %w", path, err)
      }

  However, when using a library function which returns an error, there is
  generally no way to know ahead of time whether the error will already contain
  an appropriate level of context (example: if `os.Open` returns an error, will
  the error message include the filename or not?).  This can result in duplicated
  messages during error wrapping.

  If in doubt, initially write your code such that all errors are wrapped, then
  later remove the wrapping if it turns out that some portions of the message are
  redundant.

- When returning errors, don't write full sentences as your error is likely
  to be wrapped.
