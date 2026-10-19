// @ts-check
import { dispatchToast } from '../core/events.js';

/** @typedef {{ raw: string, id: string, secret_digest: string }} GeneratedCredential */
/** @typedef {{ credential: GeneratedCredential, idempotencyKey: string, payload: Record<string, unknown> }} PendingTenantCreation */
/** @typedef {{ tenantId: string, version: number, credential: GeneratedCredential }} PendingCredentialRotation */

const emptyCreateForm = () => ({
  displayName: '', supportEmail: '', emailHost: '', emailPort: 587,
  emailUsername: '', emailPassword: '', emailFromAddress: '', smsEnabled: false,
  smsAccountSID: '', smsAuthToken: '', smsFromNumber: '',
});

function base64URL(bytes) {
  let binary = '';
  bytes.forEach((value) => { binary += String.fromCharCode(value); });
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

async function generateAPIKey() {
  const credentialID = crypto.randomUUID();
  const secret = crypto.getRandomValues(new Uint8Array(32));
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', secret));
  return {
    raw: `pgn_1_${credentialID}_${base64URL(secret)}`,
    id: credentialID,
    secret_digest: base64URL(digest),
  };
}

function isDefinitiveAPIError(error) {
  const statusCode = Number(
    error && typeof error === 'object' && 'statusCode' in error ? error.statusCode : 0,
  );
  return statusCode >= 400 && statusCode < 500;
}

export function createTenantManagement({ apiClient, strings, actions }) {
  const authStore = () => window.Alpine.store('auth');
  return {
    strings,
    actions,
    tenants: [],
    isLoading: false,
    isSubmitting: false,
    errorMessage: '',
    createForm: emptyCreateForm(),
    editForm: emptyCreateForm(),
    selectedTenant: null,
    pendingDeleteTenant: null,
    oneTimeAPIKey: '',
    oneTimeTitle: '',
    dialogTrigger: null,
    loadRequestID: 0,
    workspaceGeneration: 0,
    pendingCreate: /** @type {PendingTenantCreation | null} */ (null),
    pendingRotation: /** @type {PendingCredentialRotation | null} */ (null),
    init() {
      this.loadIfAuthenticated();
      this.$watch(
        () => authStore().isAuthenticated,
        (authenticated) => {
          if (authenticated) this.loadTenants();
          else this.resetWorkspace();
        },
      );
    },
    async loadIfAuthenticated() {
      if (authStore().isAuthenticated) await this.loadTenants();
    },
    async loadTenants() {
      const requestID = ++this.loadRequestID;
      this.isLoading = true;
      this.errorMessage = '';
      try {
        const tenants = await apiClient.listTenants();
        if (requestID !== this.loadRequestID || !authStore().isAuthenticated) return;
        this.tenants = tenants;
      } catch {
        if (requestID !== this.loadRequestID || !authStore().isAuthenticated) return;
        this.errorMessage = strings.loadError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        if (requestID === this.loadRequestID) this.isLoading = false;
      }
    },
    resetWorkspace() {
      this.workspaceGeneration += 1;
      this.loadRequestID += 1;
      if (this.pendingCreate) this.pendingCreate.credential.raw = '';
      if (this.pendingRotation) this.pendingRotation.credential.raw = '';
      this.pendingCreate = null;
      this.pendingRotation = null;
      this.tenants = [];
      this.isLoading = false;
      this.isSubmitting = false;
      this.errorMessage = '';
      this.createForm = emptyCreateForm();
      this.editForm = emptyCreateForm();
      this.selectedTenant = null;
      this.pendingDeleteTenant = null;
      this.oneTimeAPIKey = '';
      this.oneTimeTitle = '';
      this.dialogTrigger = null;
      this.$refs.createDialog?.close();
      this.$refs.editDialog?.close();
      this.$refs.keyDialog?.close();
      this.$refs.deleteDialog?.close();
    },
    openCreateDialog(event) {
      this.dialogTrigger = event?.currentTarget instanceof HTMLElement ? event.currentTarget : null;
      this.createForm = emptyCreateForm();
      this.$refs.createDialog?.showModal();
    },
    closeCreateDialog() {
      if (this.pendingCreate) this.pendingCreate.credential.raw = '';
      this.pendingCreate = null;
      this.$refs.createDialog?.close();
    },
    async createTenant(event) {
      event?.preventDefault();
      if (this.isSubmitting) return;
      this.isSubmitting = true;
      this.errorMessage = '';
      const generation = this.workspaceGeneration;
      let operation = this.pendingCreate;
      if (!operation) {
        const credential = await generateAPIKey();
        if (generation !== this.workspaceGeneration) {
          credential.raw = '';
          return;
        }
        const payload = /** @type {Record<string, unknown>} */ ({
          display_name: this.createForm.displayName,
          support_email: this.createForm.supportEmail,
          email_profile: {
            host: this.createForm.emailHost,
            port: Number(this.createForm.emailPort),
            username: this.createForm.emailUsername,
            password: this.createForm.emailPassword,
            from_address: this.createForm.emailFromAddress,
          },
          api_credential: { id: credential.id, secret_digest: credential.secret_digest },
        });
        if (this.createForm.smsEnabled) {
          payload.sms_profile = {
            account_sid: this.createForm.smsAccountSID,
            auth_token: this.createForm.smsAuthToken,
            from_number: this.createForm.smsFromNumber,
          };
        }
        operation = { credential, idempotencyKey: crypto.randomUUID(), payload };
        this.pendingCreate = operation;
      }
      try {
        await apiClient.createTenant(operation.payload, operation.idempotencyKey);
        if (generation !== this.workspaceGeneration) {
          operation.credential.raw = '';
          if (this.pendingCreate === operation) this.pendingCreate = null;
          return;
        }
        this.oneTimeAPIKey = operation.credential.raw;
        this.oneTimeTitle = strings.createdKeyTitle;
        operation.credential.raw = '';
        this.pendingCreate = null;
        this.$refs.createDialog?.close();
        dispatchToast({ variant: 'success', message: strings.createSuccess });
        this.$nextTick(() => this.$refs.keyDialog?.showModal());
        await this.loadTenants();
      } catch (error) {
        if (generation !== this.workspaceGeneration) {
          operation.credential.raw = '';
          if (this.pendingCreate === operation) this.pendingCreate = null;
          return;
        }
        if (isDefinitiveAPIError(error)) {
          operation.credential.raw = '';
          this.pendingCreate = null;
        }
        this.errorMessage = strings.createError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        if (generation === this.workspaceGeneration) this.isSubmitting = false;
      }
    },
    openEditDialog(event, resource) {
      this.dialogTrigger = event?.currentTarget instanceof HTMLElement ? event.currentTarget : null;
      this.selectedTenant = resource;
      this.editForm = {
        ...emptyCreateForm(),
        displayName: resource.displayName,
        supportEmail: resource.supportEmail,
        emailHost: resource.emailProfile?.host || '',
        emailPort: resource.emailProfile?.port || 587,
        emailFromAddress: resource.emailProfile?.from_address || '',
        smsEnabled: Boolean(resource.smsProfile),
        smsFromNumber: resource.smsProfile?.from_number || '',
      };
      this.$refs.editDialog?.showModal();
    },
    closeEditDialog() { this.$refs.editDialog?.close(); },
    async saveTenant(event) {
      event?.preventDefault();
      if (this.isSubmitting || !this.selectedTenant) return;
      this.isSubmitting = true;
      try {
        await apiClient.updateTenant(this.selectedTenant.id, this.selectedTenant.version, {
          display_name: this.editForm.displayName,
          support_email: this.editForm.supportEmail,
        });
        const emailChanges = {
          host: this.editForm.emailHost,
          port: Number(this.editForm.emailPort),
          from_address: this.editForm.emailFromAddress,
        };
        if (this.editForm.emailUsername) emailChanges.username = this.editForm.emailUsername;
        if (this.editForm.emailPassword) emailChanges.password = this.editForm.emailPassword;
        await apiClient.patchEmailProfile(this.selectedTenant.id, this.selectedTenant.emailProfile.version, emailChanges);
        if (this.editForm.smsEnabled) {
          const smsChanges = { from_number: this.editForm.smsFromNumber };
          if (this.editForm.smsAccountSID) smsChanges.account_sid = this.editForm.smsAccountSID;
          if (this.editForm.smsAuthToken) smsChanges.auth_token = this.editForm.smsAuthToken;
          if (this.selectedTenant.smsProfile) {
            await apiClient.patchSMSProfile(this.selectedTenant.id, this.selectedTenant.smsProfile.version, smsChanges);
          } else {
            await apiClient.putSMSProfile(this.selectedTenant.id, 0, smsChanges);
          }
        } else if (this.selectedTenant.smsProfile) {
          await apiClient.deleteSMSProfile(this.selectedTenant.id, this.selectedTenant.smsProfile.version);
        }
        this.closeEditDialog();
        await this.loadTenants();
        dispatchToast({ variant: 'success', message: strings.updateSuccess });
      } catch {
        this.errorMessage = strings.updateError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isSubmitting = false;
      }
    },
    async rotateCredential(event, resource) {
      if (this.isSubmitting) return;
      this.dialogTrigger = event?.currentTarget instanceof HTMLElement ? event.currentTarget : null;
      this.isSubmitting = true;
      const generation = this.workspaceGeneration;
      let operation = this.pendingRotation;
      if (!operation || operation.tenantId !== resource.id || operation.version !== resource.apiCredential.version) {
        if (operation) operation.credential.raw = '';
        const credential = await generateAPIKey();
        if (generation !== this.workspaceGeneration) {
          credential.raw = '';
          return;
        }
        operation = { tenantId: resource.id, version: resource.apiCredential.version, credential };
        this.pendingRotation = operation;
      }
      try {
        await apiClient.rotateTenantCredential(operation.tenantId, operation.version, {
          id: operation.credential.id, secret_digest: operation.credential.secret_digest,
        });
        if (generation !== this.workspaceGeneration) {
          operation.credential.raw = '';
          if (this.pendingRotation === operation) this.pendingRotation = null;
          return;
        }
        this.oneTimeAPIKey = operation.credential.raw;
        this.oneTimeTitle = strings.rotatedKeyTitle;
        operation.credential.raw = '';
        this.pendingRotation = null;
        this.$nextTick(() => this.$refs.keyDialog?.showModal());
        await this.loadTenants();
      } catch (error) {
        if (generation !== this.workspaceGeneration) {
          operation.credential.raw = '';
          if (this.pendingRotation === operation) this.pendingRotation = null;
          return;
        }
        if (isDefinitiveAPIError(error)) {
          operation.credential.raw = '';
          this.pendingRotation = null;
        }
        dispatchToast({ variant: 'error', message: strings.rotateError });
      } finally {
        if (generation === this.workspaceGeneration) this.isSubmitting = false;
      }
    },
    openDeleteDialog(event, resource) {
      this.dialogTrigger = event?.currentTarget instanceof HTMLElement ? event.currentTarget : null;
      this.pendingDeleteTenant = resource;
      this.$refs.deleteDialog?.showModal();
    },
    closeDeleteDialog() { this.$refs.deleteDialog?.close(); },
    async deleteTenant() {
      if (this.isSubmitting || !this.pendingDeleteTenant) return;
      this.isSubmitting = true;
      try {
        await apiClient.deleteTenant(this.pendingDeleteTenant.id, this.pendingDeleteTenant.version);
        this.closeDeleteDialog();
        await this.loadTenants();
        dispatchToast({ variant: 'success', message: strings.deleteSuccess });
      } catch {
        dispatchToast({ variant: 'error', message: strings.deleteError });
      } finally {
        this.isSubmitting = false;
      }
    },
    async copyAPIKey() {
      try {
        await navigator.clipboard.writeText(this.oneTimeAPIKey);
        dispatchToast({ variant: 'success', message: strings.copySuccess });
      } catch {
        dispatchToast({ variant: 'error', message: strings.copyError });
      }
    },
    closeKeyDialog() { this.$refs.keyDialog?.close(); },
    handleKeyDialogClosed() {
      this.oneTimeAPIKey = '';
      this.oneTimeTitle = '';
      this.restoreDialogFocus();
    },
    handleDialogClosed() {
      this.selectedTenant = null;
      this.pendingDeleteTenant = null;
      this.restoreDialogFocus();
    },
    handleCreateDialogClosed() {
      if (this.pendingCreate) this.pendingCreate.credential.raw = '';
      this.pendingCreate = null;
      this.handleDialogClosed();
    },
    restoreDialogFocus() {
      const trigger = this.dialogTrigger;
      this.dialogTrigger = null;
      this.$nextTick(() => {
        const target = trigger && trigger.isConnected ? trigger : this.$refs.tenantList;
        if (target instanceof HTMLElement) target.focus();
      });
    },
  };
}
