// @ts-check

/** @typedef {import('../types.d.js').SMTPCredentials} SMTPCredentials */

/**
 * Creates the credential-dialog state and clipboard behavior.
 *
 * @param {{ strings: typeof import('../constants.js').STRINGS.smtpIdentities }} options
 */
export function createSMTPCredentialsDialog(options) {
  const { strings } = options;

  return {
    credentials: /** @type {SMTPCredentials | null} */ (null),
    credentialNotice: /** @type {{ variant: string, message: string } | null} */ (null),
    credentialsDialogTrigger: /** @type {HTMLElement | null} */ (null),
    async copyCredentialValue(value, successMessage) {
      try {
        if (!navigator.clipboard || typeof navigator.clipboard.writeText !== 'function') {
          throw new Error('clipboard_unavailable');
        }
        await navigator.clipboard.writeText(String(value ?? ''));
        this.setCredentialNotice('success', successMessage);
      } catch (error) {
        this.setCredentialNotice('error', strings.copyError);
      }
    },
    setCredentialNotice(variant, message) {
      this.credentialNotice = { variant, message };
    },
    openCredentialsDialog(trigger = null) {
      if (trigger instanceof HTMLElement) {
        this.credentialsDialogTrigger = trigger;
      }
      const dialog = this.$refs.credentialsDialog;
      if (dialog && typeof dialog.showModal === 'function' && !dialog.open) {
        dialog.showModal();
      }
    },
    closeCredentialsDialog() {
      const dialog = this.$refs.credentialsDialog;
      if (dialog && typeof dialog.close === 'function') {
        dialog.close();
      }
    },
    handleCredentialsDialogClosed() {
      const trigger = this.credentialsDialogTrigger;
      this.credentialsDialogTrigger = null;
      this.credentialNotice = null;
      this.$nextTick(() => {
        if (trigger && trigger.isConnected) {
          trigger.focus();
        }
      });
    },
  };
}
