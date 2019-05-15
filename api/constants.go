package api

const (
	apiWebHookGroupPath     = "/webhook"
	apiAuthProjectGroupPath = "/api/v1"
	apiAuthUserGroupPath    = "/admin/api/v1"

	LimitDefault  = 100
	OffsetDefault = 0

	requestParameterId                       = "id"
	requestParameterName                     = "name"
	requestParameterSku                      = "sku"
	requestParameterIsSigned                 = "is_signed"
	requestParameterMerchantId               = "merchant_id"
	requestParameterProjectId                = "project_id"
	requestParameterPaymentMethodId          = "method_id"
	requestParameterOrderId                  = "order_id"
	requestParameterRefundId                 = "refund_id"
	requestParameterNotificationId           = "notification_id"
	requestParameterUserId                   = "user"
	requestParameterLimit                    = "limit"
	requestParameterOffset                   = "offset"
	requestParameterFile                     = "file"
	requestParameterUtmSource                = "utm_source"
	requestParameterUtmMedium                = "utm_medium"
	requestParameterUtmCampaign              = "utm_campaign"
	requestParameterIsSystem                 = "is_system"
	requestParameterAgreementType            = "agreement_type"
	requestParameterHasMerchantSignature     = "has_merchant_signature"
	requestParameterHasPspSignature          = "has_psp_signature"
	requestParameterAgreementSentViaMail     = "agreement_sent_via_mail"
	requestParameterMailTrackingLink         = "mail_tracking_link"
	requestParameterImage                    = "image"
	requestParameterCallbackCurrency         = "callback_currency"
	requestParameterCallbackProtocol         = "callback_protocol"
	requestParameterCreateOrderAllowedUrls   = "create_order_allowed_urls"
	requestParameterAllowDynamicNotifyUrls   = "allow_dynamic_notify_urls"
	requestParameterAllowDynamicRedirectUrls = "allow_dynamic_redirect_urls"
	requestParameterLimitsCurrency           = "limits_currency"
	requestParameterMinPaymentAmount         = "min_payment_amount"
	requestParameterMaxPaymentAmount         = "max_payment_amount"
	requestParameterNotifyEmails             = "notify_emails"
	requestParameterIsProductsCheckout       = "is_products_checkout"
	requestParameterSecretKey                = "secret_key"
	requestParameterSignatureRequired        = "signature_required"
	requestParameterSendNotifyEmail          = "send_notify_email"
	requestParameterUrlCheckAccount          = "url_check_account"
	requestParameterUrlProcessPayment        = "url_process_payment"
	requestParameterUrlRedirectFail          = "url_redirect_fail"
	requestParameterUrlRedirectSuccess       = "url_redirect_success"
	requestParameterStatus                   = "status"
	requestAuthorizationTokenRegex           = "Bearer ([A-z0-9_.-]{10,})"

	errorIdIsEmpty                                    = "identifier can't be empty"
	errorIncorrectMerchantId                          = "incorrect merchant identifier"
	errorIncorrectProjectId                           = "incorrect project identifier"
	errorIncorrectNotificationId                      = "incorrect notification identifier"
	errorIncorrectOrderId                             = "incorrect order identifier"
	errorIncorrectPaymentMethodId                     = "incorrect payment method identifier"
	errorIncorrectProductId                           = "incorrect product identifier"
	errorIncorrectPaylinkId                           = "incorrect paylink identifier"
	errorMessageAccessDenied                          = "access denied"
	errorMessageAuthorizationHeaderNotFound           = "authorization header not found"
	errorMessageAuthorizationTokenNotFound            = "authorization token not found"
	errorMessageAuthorizedUserNotFound                = "information about authorized user not found"
	errorMessageMask                                  = "field validation for '%s' failed on the '%s' tag"
	errorQueryParamsIncorrect                         = "incorrect query parameters"
	errorUnknown                                      = "unknown error. try request later"
	errorMessageAgreementNotGenerated                 = "agreement for merchant not generated early"
	errorMessageAgreementNotFound                     = "agreement for merchant not found"
	errorMessageAgreementUploadMaxSize                = "agreement document max upload size can't be greater than %d"
	errorMessageAgreementContentType                  = "agreement document type must be a pdf"
	errorMessageAgreementCanNotBeGenerate             = "agreement can't be generated for not checked merchant data"
	errorMessageAgreementTypeIncorrectType            = "agreement type parameter have incorrect type"
	errorMessageHasMerchantSignatureIncorrectType     = "merchant signature parameter has incorrect type"
	errorMessageHasPspSignatureIncorrectType          = "paysuper signature parameter has incorrect type"
	errorMessageAgreementSentViaMailIncorrectType     = "agreement sent via email parameter has incorrect type"
	errorMessageMailTrackingLinkIncorrectType         = "mail tracking link parameter has incorrect type"
	errorMessageNameIncorrectType                     = "name parameter has incorrect type"
	errorMessageImageIncorrectType                    = "image parameter has incorrect type"
	errorMessageCallbackCurrencyIncorrectType         = "callback currency parameter has incorrect type"
	errorMessageCallbackProtocolIncorrectType         = "callback protocol parameter has incorrect type"
	errorMessageCreateOrderAllowedUrlsIncorrectType   = "create order allowed urls parameter has incorrect type"
	errorMessageAllowDynamicNotifyUrlsIncorrectType   = "allow dynamic notify urls parameter has incorrect type"
	errorMessageAllowDynamicRedirectUrlsIncorrectType = "allow dynamic redirect urls parameter has incorrect type"
	errorMessageLimitsCurrencyIncorrectType           = "limits currency parameter has incorrect type"
	errorMessageMinPaymentAmountIncorrectType         = "min payment amount parameter has incorrect type"
	errorMessageMaxPaymentAmountIncorrectType         = "max payment amount parameter has incorrect type"
	errorMessageNotifyEmailsIncorrectType             = "notify emails parameter has incorrect type"
	errorMessageIsProductsCheckoutIncorrectType       = "is products checkout parameter has incorrect type"
	errorMessageSecretKeyIncorrectType                = "secret key parameter has incorrect type"
	errorMessageSignatureRequiredIncorrectType        = "signature required parameter has incorrect type"
	errorMessageSendNotifyEmailIncorrectType          = "send notify email parameter has incorrect type"
	errorMessageUrlCheckAccountIncorrectType          = "url check account parameter has incorrect type"
	errorMessageUrlProcessPaymentIncorrectType        = "url process payment parameter has incorrect type"
	errorMessageUrlRedirectFailIncorrectType          = "url redirect fail parameter has incorrect type"
	errorMessageUrlRedirectSuccessIncorrectType       = "url redirect success parameter has incorrect type"
	errorMessageStatusIncorrectType                   = "status parameter has incorrect type"
	errorMessageSignatureHeaderIsEmpty                = "header with request signature can't be empty"

	HeaderAcceptLanguage      = "Accept-Language"
	HeaderUserAgent           = "User-Agent"
	HeaderXApiSignatureHeader = "X-API-SIGNATURE"

	EnvironmentProduction        = "prod"
	CustomerTokenCookiesName     = "_ps_ctkn"
	CustomerTokenCookiesLifetime = 2592000

	agreementPageTemplateName = "agreement.html"
)