// @ts-check

/**
 * @typedef {"queued" | "sent" | "errored" | "cancelled"} NotificationStatusKey
 */

/**
 * @typedef {Object} TenantOption
 * @property {string} id
 * @property {string} displayName
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
 * @typedef {Object} NotificationListPage
 * @property {NotificationItem[]} notifications
 * @property {string} nextCursor
 */

/**
 * @typedef {Object} NotificationListOptions
 * @property {string=} query
 * @property {string=} cursor
 * @property {number=} limit
 */

/**
 * @typedef {Object} SMTPIdentity
 * @property {string} id
 * @property {string} emailAddress
 * @property {string} username
 * @property {string[]} forwardTo
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
 * @typedef {Object} SMTPDomainDNSRecord
 * @property {string} type
 * @property {string} host
 * @property {string} value
 * @property {string} purpose
 */

/**
 * @typedef {Object} SMTPDomainDNSCheck
 * @property {string} type
 * @property {string} host
 * @property {string} expected
 * @property {boolean} passed
 * @property {string} message
 */

/**
 * @typedef {Object} SMTPSenderDomain
 * @property {number} id
 * @property {string} domain
 * @property {string} status
 * @property {SMTPDomainDNSRecord[]} dnsRecords
 * @property {SMTPDomainDNSCheck[]} dnsChecks
 * @property {string | null} lastCheckedAt
 * @property {string} createdAt
 * @property {string} updatedAt
 */

/**
 * @typedef {Object} StatusOption
 * @property {NotificationStatusKey | "all"} value
 * @property {string} label
 */

/** @typedef {'delivered' | 'delayed' | 'bounced' | 'complaint' | 'rejected' | 'rendering_failed' | 'unconfirmed'} EmailDeliveryStatus */
/**
 * @typedef {Object} EmailDeliveryRecipient
 * @property {string} address
 * @property {EmailDeliveryStatus} status
 * @property {string} updatedAt
 * @property {string} response
 * @property {string} deliveredAt
 */
/**
 * @typedef {Object} EmailDeliveryMessage
 * @property {string} id
 * @property {string} from
 * @property {string} subject
 * @property {string} sentAt
 * @property {string} updatedAt
 * @property {string} expiresAt
 * @property {EmailDeliveryRecipient[]} recipients
 * @property {string} parentMessageId
 * @property {string[]} forwardedMessageIds
 */
/**
 * @typedef {Object} EmailDeliveryPage
 * @property {EmailDeliveryMessage[]} messages
 * @property {string} nextCursor
 * @property {string} windowStart
 * @property {string} asOf
 */
/**
 * @typedef {Object} EmailDeliveryListOptions
 * @property {string=} query
 * @property {string=} cursor
 * @property {number=} limit
 * @property {AbortSignal=} signal
 */
