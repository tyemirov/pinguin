// @ts-check
import { dispatchToast } from '../core/events.js';

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
    init() {
      this.loadIfAuthenticated();
      this.$watch(
        () => authStore().isAuthenticated,
        (authenticated) => {
          if (authenticated) this.loadTenants();
          else this.tenants = [];
        },
      );
    },
    async loadIfAuthenticated() {
      if (authStore().isAuthenticated) await this.loadTenants();
    },
    async loadTenants() {
      this.isLoading = true;
      this.errorMessage = '';
      try {
        this.tenants = await apiClient.listTenants();
      } catch {
        this.errorMessage = strings.loadError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isLoading = false;
      }
    },
    openCreateDialog(event) {
      this.dialogTrigger = event?.currentTarget instanceof HTMLElement ? event.currentTarget : null;
      this.createForm = emptyCreateForm();
      this.$refs.createDialog?.showModal();
    },
    closeCreateDialog() { this.$refs.createDialog?.close(); },
    async createTenant(event) {
      event?.preventDefault();
      if (this.isSubmitting) return;
      this.isSubmitting = true;
      this.errorMessage = '';
      const credential = await generateAPIKey();
      try {
        const payload = {
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
        };
        if (this.createForm.smsEnabled) {
          payload.sms_profile = {
            account_sid: this.createForm.smsAccountSID,
            auth_token: this.createForm.smsAuthToken,
            from_number: this.createForm.smsFromNumber,
          };
        }
        await apiClient.createTenant(payload, crypto.randomUUID());
        this.oneTimeAPIKey = credential.raw;
        this.oneTimeTitle = strings.createdKeyTitle;
        this.closeCreateDialog();
        await this.loadTenants();
        dispatchToast({ variant: 'success', message: strings.createSuccess });
        this.$nextTick(() => this.$refs.keyDialog?.showModal());
      } catch {
        credential.raw = '';
        this.oneTimeAPIKey = '';
        this.errorMessage = strings.createError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isSubmitting = false;
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
      const credential = await generateAPIKey();
      try {
        await apiClient.rotateTenantCredential(resource.id, resource.apiCredential.version, {
          id: credential.id, secret_digest: credential.secret_digest,
        });
        this.oneTimeAPIKey = credential.raw;
        this.oneTimeTitle = strings.rotatedKeyTitle;
        await this.loadTenants();
        this.$nextTick(() => this.$refs.keyDialog?.showModal());
      } catch {
        credential.raw = '';
        this.oneTimeAPIKey = '';
        dispatchToast({ variant: 'error', message: strings.rotateError });
      } finally {
        this.isSubmitting = false;
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
