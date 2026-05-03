// @ts-check

/**
 * @typedef {"queued" | "sent" | "errored" | "cancelled"} NotificationStatusKey
 */

/**
 * @typedef {Object} NotificationItem
 * @property {string} id
 * @property {string} tenantId
 * @property {string} type
 * @property {string} recipient
 * @property {string} subject
 * @property {string} message
 * @property {NotificationStatusKey} status
 * @property {string} createdAt
 * @property {string} updatedAt
 * @property {string | null} scheduledFor
 * @property {number} retryCount
 */

/**
 * @typedef {Object} SMTPIdentity
 * @property {string} id
 * @property {string} emailAddress
 * @property {string} username
 * @property {string} status
 * @property {string | null} lastUsedAt
 * @property {string} createdAt
 * @property {string} updatedAt
 */

/**
 * @typedef {Object} SMTPCredentials
 * @property {SMTPIdentity} identity
 * @property {{ host: string, port: number, securityMode: string }} smtpSettings
 * @property {string} username
 * @property {string} password
 */

/**
 * @typedef {Object} StatusOption
 * @property {NotificationStatusKey | "all"} value
 * @property {string} label
 */
