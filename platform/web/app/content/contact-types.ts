// Private contact follow-up (platform/contracts/openapi.yaml, WEB-007).
export type ContactStatus = "pending" | "processing" | "retry_wait" | "accepted_by_provider" | "failed" | "unknown";

export const contactStatusLabels: Record<ContactStatus, string> = {
  pending: "Pendiente",
  processing: "Enviando",
  retry_wait: "Pendiente de reintento",
  accepted_by_provider: "Aceptado por Resend",
  failed: "Fallido",
  unknown: "Resultado incierto",
};

export const contactStatusHelp: Record<ContactStatus, string> = {
  pending: "Guardado; el envío a Resend empieza en segundos.",
  processing: "Un envío está en curso.",
  retry_wait: "Resend no confirmó el envío; se reintenta solo con la misma clave.",
  accepted_by_provider: "Resend lo aceptó. No es una confirmación de entrega en tu buzón.",
  failed: "No se envió. Revisa el error: suele ser configuración (dominio, clave o remitente).",
  unknown: "No se sabe si Resend lo recibió. Búscalo en Resend antes de reintentar: podría duplicarse.",
};

export type ContactSummary = { id: string; received_at: string; locale: "es" | "en"; name: string; email: string; status: ContactStatus; attempts: number };

export type ContactDetail = ContactSummary & {
  message: string;
  version: number;
  max_attempts: number;
  next_attempt_at: string | null;
  first_attempt_at: string | null;
  window_ends_at: string | null;
  uncertain: boolean;
  provider_email_id: string | null;
  last_error: string | null;
  idempotency_key: string;
  retry: "none" | "same_key" | "new_key_confirm";
  attempt_log: { attempt: number; at: string; idempotency_key: string; outcome: string; http_status: number | null; error_name: string | null; provider_request_id: string | null }[];
};

export const outcomeLabels: Record<string, string> = {
  accepted: "Aceptado",
  transient: "Rechazo temporal",
  uncertain: "Incierto",
  permanent: "Rechazo permanente",
  lease_expired: "Interrumpido (caída o reinicio)",
};
