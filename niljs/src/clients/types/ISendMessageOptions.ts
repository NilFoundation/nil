/**
 * The options for sending a message.
 */
type ISendMessageOptions = {
  /**
   * If true, the message will be validated before sending.
   * If the message is invalid, an error will be thrown.
   * If false, the message will not be validated before sending.
   * @default true
   */
  shouldValidate?: boolean;
};

export type { ISendMessageOptions };
