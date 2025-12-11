// @ts-check

/**
 * Playwright global setup to tolerate broken pipe errors that can occur when
 * stdout/stderr pipes are closed by the runner (e.g., timeout wrappers) while
 * the test runner or dev server is still emitting logs.
 */
module.exports = async function globalSetup() {
  const swallowEpipe = (stream) => {
    if (!stream) {
      return;
    }
    stream.on('error', (error) => {
      if (error && error.code === 'EPIPE') {
        return;
      }
      throw error;
    });
  };

  swallowEpipe(process.stdout);
  swallowEpipe(process.stderr);
};
