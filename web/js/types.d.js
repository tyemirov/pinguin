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
 * @typedef {Object} StatusOption
 * @property {NotificationStatusKey | "all"} value
 * @property {string} label
 */
