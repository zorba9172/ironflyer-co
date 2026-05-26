// Hand-written types + Apollo documents for the P1 notification surface.
//
// The orchestrator notification schema is being implemented in parallel
// in core/orchestrator/. Until those resolvers land in
// core/orchestrator/internal/operations/graph/schema/*.graphql, codegen
// can't generate typed hooks for these operations. This module is the
// stand-in: it mirrors the frozen GraphQL contract from the task brief
// exactly, exposes typed `gql` documents the bell + settings page bind
// to, and stays drop-in-replaceable when the schema merges (callers
// will swap the import path from "@/lib/gql/notifications.types" to
// "@/lib/gql/__generated__" without touching component code).
//
// Runtime safety: the bell + settings page query through a
// `notificationsSchemaAvailable()` check that swallows
// `Cannot query field "notifications"` style server errors so an
// in-flight backend deploy never breaks the cockpit.

import { gql, type DocumentNode } from "@apollo/client";

// --------------------------------------------------------------------
// Type contract (frozen) — must stay in lockstep with the GraphQL
// schema declared by the orchestrator.
// --------------------------------------------------------------------

export interface Notification {
  __typename?: "Notification";
  id: string;
  kind: string;
  title: string;
  body: string;
  link: string | null;
  severity: string;
  readAt: string | null;
  createdAt: string;
}

export interface ChannelPref {
  __typename?: "ChannelPref";
  email: boolean;
  inApp: boolean;
}

export interface NotificationPreferences {
  __typename?: "NotificationPreferences";
  userId: string;
  pauseAll: boolean;
  onRunComplete: ChannelPref;
  onGateFailed: ChannelPref;
  onDeployDone: ChannelPref;
  onBudgetWarning: ChannelPref;
  onReceipt: ChannelPref;
}

export interface ChannelPrefInput {
  email: boolean;
  inApp: boolean;
}

export interface NotificationPreferencesInput {
  pauseAll?: boolean;
  onRunComplete?: ChannelPrefInput;
  onGateFailed?: ChannelPrefInput;
  onDeployDone?: ChannelPrefInput;
  onBudgetWarning?: ChannelPrefInput;
  onReceipt?: ChannelPrefInput;
}

// --------------------------------------------------------------------
// Operation payloads
// --------------------------------------------------------------------

export interface NotificationsQueryVariables {
  unreadOnly?: boolean | null;
}

export interface NotificationsQuery {
  __typename?: "Query";
  notifications: Notification[];
  unreadNotificationCount: number;
}

export interface NotificationPreferencesQueryVariables {
  // intentionally empty — kept as a placeholder for parity with codegen.
  _placeholder?: never;
}

export interface NotificationPreferencesQuery {
  __typename?: "Query";
  notificationPreferences: NotificationPreferences;
}

export interface MarkNotificationReadVariables {
  id: string;
}

export interface MarkNotificationReadMutation {
  __typename?: "Mutation";
  markNotificationRead: Pick<Notification, "id" | "readAt"> & {
    __typename?: "Notification";
  };
}

export interface MarkAllNotificationsReadVariables {
  _placeholder?: never;
}

export interface MarkAllNotificationsReadMutation {
  __typename?: "Mutation";
  markAllNotificationsRead: number;
}

export interface UpdateNotificationPreferencesVariables {
  input: NotificationPreferencesInput;
}

export interface UpdateNotificationPreferencesMutation {
  __typename?: "Mutation";
  updateNotificationPreferences: NotificationPreferences;
}

export interface NotificationStreamSubscriptionVariables {
  _placeholder?: never;
}

export interface NotificationStreamSubscription {
  __typename?: "Subscription";
  notificationStream: Notification;
}

// --------------------------------------------------------------------
// gql documents
// --------------------------------------------------------------------

export const NotificationsDocument: DocumentNode = gql`
  query Notifications($unreadOnly: Boolean) {
    notifications(unreadOnly: $unreadOnly) {
      id
      kind
      title
      body
      link
      severity
      readAt
      createdAt
    }
    unreadNotificationCount
  }
`;

export const NotificationPreferencesDocument: DocumentNode = gql`
  query NotificationPreferences {
    notificationPreferences {
      userId
      pauseAll
      onRunComplete {
        email
        inApp
      }
      onGateFailed {
        email
        inApp
      }
      onDeployDone {
        email
        inApp
      }
      onBudgetWarning {
        email
        inApp
      }
      onReceipt {
        email
        inApp
      }
    }
  }
`;

export const MarkNotificationReadDocument: DocumentNode = gql`
  mutation MarkNotificationRead($id: ID!) {
    markNotificationRead(id: $id) {
      id
      readAt
    }
  }
`;

export const MarkAllNotificationsReadDocument: DocumentNode = gql`
  mutation MarkAllNotificationsRead {
    markAllNotificationsRead
  }
`;

export const UpdateNotificationPreferencesDocument: DocumentNode = gql`
  mutation UpdateNotificationPreferences(
    $input: NotificationPreferencesInput!
  ) {
    updateNotificationPreferences(input: $input) {
      userId
      pauseAll
      onRunComplete {
        email
        inApp
      }
      onGateFailed {
        email
        inApp
      }
      onDeployDone {
        email
        inApp
      }
      onBudgetWarning {
        email
        inApp
      }
      onReceipt {
        email
        inApp
      }
    }
  }
`;

export const NotificationStreamDocument: DocumentNode = gql`
  subscription NotificationStream {
    notificationStream {
      id
      kind
      title
      body
      link
      severity
      readAt
      createdAt
    }
  }
`;

// --------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------

// Apollo errors come back through `errors` on the result OR as a thrown
// ApolloError with graphQLErrors[]. Either way the validation message
// for "field is not defined" includes the words "Cannot query field".
// We use that to detect the not-yet-deployed state and skip the
// notification surface gracefully instead of painting a red banner.
export function isSchemaMissing(err: unknown): boolean {
  if (!err) return false;
  const msg =
    typeof err === "string"
      ? err
      : err instanceof Error
        ? err.message
        : "";
  if (!msg) return false;
  return (
    msg.includes("Cannot query field") ||
    msg.includes("Unknown type \"NotificationPreferencesInput\"") ||
    msg.includes("notifications") && msg.includes("not defined") ||
    msg.includes("notificationStream")
  );
}
