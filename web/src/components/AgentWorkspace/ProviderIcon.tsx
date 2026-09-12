// Provider brand logos for the chat input model dropdown + LLM settings
// cards. Each component renders the brand's official logomark (sourced from
// simple-icons, CC0-licensed) inside a circular brand-coloured badge — same
// shape as the OpenRouter / Poe model lists. Sized via `size` (default 16).

import type { CSSProperties } from 'react';
import {useId} from 'react';

type ProviderIconProps = {
  size?: number;
  className?: string;
  style?: CSSProperties;
};

// OpenAI — black circle with the official "blossom" knot mark.
// Path data from simple-icons/openai (CC0).
export function OpenAIIcon({ size = 16, className, style }: ProviderIconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      className={className}
      style={style}
      aria-hidden
    >
      <circle cx="16" cy="16" r="16" fill="#000000" />
      <g transform="translate(4 4) scale(1)">
        <path
          fill="#ffffff"
          d="M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.0966 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729zm-9.022 12.6081a4.4755 4.4755 0 0 1-2.8764-1.0408l.1419-.0804 4.7783-2.7582a.7948.7948 0 0 0 .3927-.6813v-6.7369l2.02 1.1686a.071.071 0 0 1 .038.052v5.5826a4.504 4.504 0 0 1-4.4945 4.4944zm-9.6607-4.1254a4.4708 4.4708 0 0 1-.5346-3.0137l.142.0852 4.783 2.7582a.7712.7712 0 0 0 .7806 0l5.8428-3.3685v2.3324a.0804.0804 0 0 1-.0332.0615L9.74 19.9502a4.4992 4.4992 0 0 1-6.1408-1.6464zM2.3408 7.8956a4.485 4.485 0 0 1 2.3655-1.9728V11.6a.7664.7664 0 0 0 .3879.6765l5.8144 3.3543-2.0201 1.1685a.0757.0757 0 0 1-.071 0l-4.8303-2.7865A4.504 4.504 0 0 1 2.3408 7.872zm16.5963 3.8558L13.1038 8.364 15.1192 7.2a.0757.0757 0 0 1 .071 0l4.8303 2.7913a4.4944 4.4944 0 0 1-.6765 8.1042v-5.6772a.79.79 0 0 0-.407-.667zm2.0107-3.0231l-.142-.0852-4.7735-2.7818a.7759.7759 0 0 0-.7854 0L9.409 9.2297V6.8974a.0662.0662 0 0 1 .0284-.0615l4.8303-2.7866a4.4992 4.4992 0 0 1 6.6802 4.66zM8.3065 12.863l-2.02-1.1638a.0804.0804 0 0 1-.038-.0567V6.0742a4.4992 4.4992 0 0 1 7.3757-3.4537l-.142.0805L8.704 5.459a.7948.7948 0 0 0-.3927.6813zm1.0976-2.3654l2.602-1.4998 2.6069 1.4998v2.9994l-2.5974 1.4997-2.6067-1.4997Z"
        />
      </g>
    </svg>
  );
}

// Anthropic / Claude — clay (#D97757) circle with the official Claude
// "sunburst" mark in white. Path from simple-icons/claude (CC0).
export function AnthropicIcon({ size = 16, className, style }: ProviderIconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      className={className}
      style={style}
      aria-hidden
    >
      <circle cx="16" cy="16" r="16" fill="#d97757" />
      <g transform="translate(4 4) scale(1)">
        <path
          fill="#ffffff"
          d="m4.7144 15.9555 4.7174-2.6471.079-.2307-.079-.1275h-.2307l-.7893-.0486-2.6956-.0729-2.3375-.0971-2.2646-.1214-.5707-.1215-.5343-.7042.0546-.3522.4797-.3218.686.0608 1.5179.1032 2.2767.1578 1.6514.0972 2.4468.255h.3886l.0546-.1579-.1336-.0971-.1032-.0972L6.973 9.8356l-2.55-1.6879-1.3356-.9714-.7225-.4918-.3643-.4614-.1578-1.0078.6557-.7225.8803.0607.2246.0607.8925.686 1.9064 1.4754 2.4893 1.8336.3643.3035.1457-.1032.0182-.0728-.164-.2733-1.3539-2.4467-1.445-2.4893-.6435-1.032-.17-.6194c-.0607-.255-.1032-.4674-.1032-.7285L6.287.1335 6.6997 0l.9957.1336.419.3642.6192 1.4147 1.0018 2.2282 1.5543 3.0296.4553.8985.2429.8318.091.255h.1579v-.1457l.1275-1.706.2368-2.0947.2307-2.6957.0789-.7589.3764-.9107.7468-.4918.5828.2793.4797.686-.0668.4433-.2853 1.8517-.5586 2.9021-.3643 1.9429h.2125l.2429-.2429.9835-1.3053 1.6514-2.0643.7286-.8196.85-.9046.5464-.4311h1.0321l.759 1.1293-.34 1.1657-1.0625 1.3478-.8804 1.1414-1.2628 1.7-.7893 1.36.0729.1093.1882-.0183 2.8535-.607 1.5421-.2794 1.8396-.3157.8318.3886.091.3946-.3278.8075-1.967.4857-2.3072.4614-3.4364.8136-.0425.0304.0486.0607 1.5482.1457.6618.0364h1.621l3.0175.2247.7892.522.4736.6376-.079.4857-1.2142.6193-1.6393-.3886-3.825-.9107-1.3113-.3279h-.1822v.1093l1.0929 1.0686 2.0035 1.8092 2.5075 2.3314.1275.5768-.3218.4554-.34-.0486-2.2039-1.6575-.85-.7468-1.9246-1.621h-.1275v.17l.4432.6496 2.3436 3.5214.1214 1.0807-.17.3521-.6071.2125-.6679-.1214-1.3721-1.9246L14.38 17.959l-1.1414-1.9428-.1397.079-.674 7.2552-.3156.3703-.7286.2793-.6071-.4614-.3218-.7468.3218-1.4753.3886-1.9246.3157-1.53.2853-1.9004.17-.6314-.0121-.0425-.1397.0182-1.4328 1.9672-2.1796 2.9446-1.7243 1.8456-.4128.164-.7164-.3704.0667-.6618.4008-.5889 2.386-3.0357 1.4389-1.882.929-1.0868-.0062-.1579h-.0546l-6.3385 4.1164-1.1293.1457-.4857-.4554.0608-.7467.2307-.2429 1.9064-1.3114Z"
        />
      </g>
    </svg>
  );
}

// Zhipu (智谱 GLM / Z.ai) — black circle with a clean white "Z" stroke.
// Zhipu has no simple-icons entry; we draw a symmetric Z matching their mark.
export function ZhipuIcon({ size = 16, className, style }: ProviderIconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      className={className}
      style={style}
      aria-hidden
    >
      <circle cx="16" cy="16" r="16" fill="#0b0b0b" />
      <path
        d="M10 10 H22 L10 22 H22"
        fill="none"
        stroke="#ffffff"
        strokeWidth={3}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

// Google Gemini — white circle with the official 4-point sparkle in a
// blue→purple→pink gradient. Path from simple-icons/googlegemini (CC0).
export function GeminiIcon({ size = 16, className, style }: ProviderIconProps) {
  const gradientId=useId();
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      className={className}
      style={style}
      aria-hidden
    >
      <defs>
        <linearGradient id={gradientId} x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#4285f4" />
          <stop offset="55%" stopColor="#9168c0" />
          <stop offset="100%" stopColor="#d96570" />
        </linearGradient>
      </defs>
      <circle cx="16" cy="16" r="16" fill="#ffffff" />
      <g transform="translate(4 4) scale(1)">
        <path
          fill={`url(#${gradientId})`}
          d="M11.04 19.32Q12 21.51 12 24q0-2.49.93-4.68.96-2.19 2.58-3.81t3.81-2.55Q21.51 12 24 12q-2.49 0-4.68-.93a12.3 12.3 0 0 1-3.81-2.58 12.3 12.3 0 0 1-2.58-3.81Q12 2.49 12 0q0 2.49-.96 4.68-.93 2.19-2.55 3.81a12.3 12.3 0 0 1-3.81 2.58Q2.49 12 0 12q2.49 0 4.68.96 2.19.93 3.81 2.55t2.55 3.81"
        />
      </g>
    </svg>
  );
}

// DeepSeek — blue (#5786FE) circle with the official white whale mark.
// Path from simple-icons/deepseek (CC0).
export function DeepSeekIcon({ size = 16, className, style }: ProviderIconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      className={className}
      style={style}
      aria-hidden
    >
      <circle cx="16" cy="16" r="16" fill="#5786fe" />
      <g transform="translate(4 4) scale(1)">
        <path
          fill="#ffffff"
          d="M23.748 4.651c-.254-.124-.364.113-.512.233-.051.04-.094.09-.137.137-.372.397-.806.657-1.373.626-.829-.046-1.537.214-2.163.848-.133-.782-.575-1.248-1.247-1.548-.352-.155-.708-.311-.955-.65-.172-.24-.219-.509-.305-.774-.055-.16-.11-.323-.293-.35-.2-.031-.278.136-.356.276-.313.572-.434 1.202-.422 1.84.027 1.436.633 2.58 1.838 3.393.137.094.172.187.129.323-.082.28-.18.553-.266.833-.055.179-.137.218-.328.14a5.5 5.5 0 0 1-1.737-1.179c-.857-.828-1.631-1.743-2.597-2.46a12 12 0 0 0-.689-.47c-.985-.957.13-1.743.387-1.836.27-.098.094-.433-.778-.428-.872.003-1.67.295-2.687.685a3 3 0 0 1-.465.136 9.6 9.6 0 0 0-2.883-.101c-1.885.21-3.39 1.1-4.497 2.622C.082 8.776-.231 10.854.152 13.02c.403 2.284 1.568 4.175 3.36 5.653 1.857 1.533 3.997 2.284 6.438 2.14 1.482-.085 3.132-.284 4.994-1.86.47.234.962.328 1.78.398.629.058 1.235-.031 1.705-.129.735-.155.684-.836.418-.961-2.155-1.004-1.682-.595-2.112-.926 1.095-1.295 2.768-3.598 3.284-6.733.05-.346.115-.834.108-1.114-.004-.171.035-.238.23-.257a4.2 4.2 0 0 0 1.545-.475c1.397-.763 1.96-2.016 2.093-3.517.02-.23-.004-.467-.247-.588M11.58 18.168c-2.088-1.642-3.101-2.183-3.52-2.16-.39.024-.32.472-.234.763.09.288.207.487.371.74.114.167.192.416-.113.603-.673.416-1.842-.14-1.897-.168-1.361-.801-2.5-1.86-3.301-3.306-.775-1.393-1.225-2.888-1.299-4.482-.02-.385.094-.522.477-.592a4.7 4.7 0 0 1 1.53-.038c2.131.311 3.946 1.264 5.467 2.774.868.86 1.525 1.887 2.202 2.89.72 1.066 1.494 2.082 2.48 2.915.348.291.626.513.892.677-.802.09-2.14.109-3.055-.615zm1.001-6.44a.306.306 0 0 1 .415-.287.3.3 0 0 1 .113.074.3.3 0 0 1 .086.214c0 .17-.136.307-.308.307a.303.303 0 0 1-.306-.307m3.11 1.596c-.2.081-.4.151-.591.16a1.25 1.25 0 0 1-.798-.254c-.274-.23-.47-.358-.551-.758a1.7 1.7 0 0 1 .015-.588c.07-.327-.007-.537-.238-.727-.188-.156-.426-.199-.689-.199a.6.6 0 0 1-.254-.078.253.253 0 0 1-.114-.358 1 1 0 0 1 .192-.21c.356-.202.767-.136 1.146.016.352.144.618.408 1.001.782.392.451.462.576.685.915.176.264.336.536.446.848.066.194-.02.353-.25.45"
        />
      </g>
    </svg>
  );
}

// Kimi (Moonshot AI) — magenta circle with the official layered waveform
// mark in white. Path from simple-icons/moonshotai (CC0).
export function KimiIcon({ size = 16, className, style }: ProviderIconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      className={className}
      style={style}
      aria-hidden
    >
      <circle cx="16" cy="16" r="16" fill="#f0419e" />
      <g transform="translate(4 4) scale(1)">
        <path
          fill="#ffffff"
          d="m1.053 16.91 9.538 2.55a21 20.981 0 0 0 .06 2.031l5.956 1.592a12 11.99 0 0 1-15.554-6.172m-1.02-5.79 11.352 3.035a21 20.981 0 0 0-.469 2.01l10.817 2.89a12 11.99 0 0 1-1.845 2.004L.658 15.918a12 11.99 0 0 1-.625-4.796m1.593-5.146L13.573 9.17a21 20.981 0 0 0-1.01 1.874l11.297 3.02a21 20.981 0 0 1-.67 2.362l-11.55-3.087L.125 10.26a12 11.99 0 0 1 1.499-4.285ZM6.067 1.58l11.285 3.016a21 20.981 0 0 0-1.688 1.719l7.824 2.091a21 20.981 0 0 1 .513 2.664L2.107 5.218a12 11.99 0 0 1 3.96-3.638M21.68 4.866 7.222 1.003A12 11.99 0 0 1 21.68 4.866"
        />
      </g>
    </svg>
  );
}

// Generic fallback for an unknown provider id — zinc circle, white dot.
export function GenericProviderIcon({ size = 16, className, style }: ProviderIconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      className={className}
      style={style}
      aria-hidden
    >
      <circle cx="16" cy="16" r="16" fill="#52525b" />
      <circle cx="16" cy="16" r="4" fill="#ffffff" />
    </svg>
  );
}

// resolveBrand picks the right brand badge using the model name first
// (so a glm-* model routed through an OpenAI-compatible provider still
// shows the Zhipu badge), falling back to the provider id, then the
// generic dot. Knowing the actual model is more reliable than the
// provider config — many shops bridge non-OpenAI models through an
// OpenAI-compatible gateway.
type Brand = 'openai' | 'anthropic' | 'zhipu' | 'gemini' | 'deepseek' | 'kimi' | 'generic';

function brandFromModel(modelName: string): Brand | null {
  const n = (modelName || '').toLowerCase();
  if (n.startsWith('gpt-') || n.startsWith('o1') || n.startsWith('o3') || n.includes('davinci')) return 'openai';
  if (n.startsWith('claude') || n.startsWith('anthropic')) return 'anthropic';
  if (n.startsWith('glm') || n.startsWith('chatglm') || n.includes('cogview')) return 'zhipu';
  if (n.startsWith('gemini') || n.startsWith('palm') || n.startsWith('bison')) return 'gemini';
  if (n.startsWith('deepseek')) return 'deepseek';
  if (n.startsWith('moonshot') || n.startsWith('kimi')) return 'kimi';
  return null;
}

function brandFromProvider(provider: string): Brand {
  switch ((provider || '').toLowerCase()) {
    case 'openai':
      return 'openai';
    case 'anthropic':
    case 'claude':
      return 'anthropic';
    case 'zhipu':
    case 'glm':
    case 'bigmodel':
      return 'zhipu';
    case 'gemini':
    case 'google':
      return 'gemini';
    case 'deepseek':
      return 'deepseek';
    case 'kimi':
    case 'moonshot':
      return 'kimi';
    default:
      return 'generic';
  }
}

function renderBrand(brand: Brand, props: ProviderIconProps) {
  switch (brand) {
    case 'openai':
      return <OpenAIIcon {...props} />;
    case 'anthropic':
      return <AnthropicIcon {...props} />;
    case 'zhipu':
      return <ZhipuIcon {...props} />;
    case 'gemini':
      return <GeminiIcon {...props} />;
    case 'deepseek':
      return <DeepSeekIcon {...props} />;
    case 'kimi':
      return <KimiIcon {...props} />;
    default:
      return <GenericProviderIcon {...props} />;
  }
}

// ProviderIcon dispatches by provider id to the correct badge. Useful
// when only a provider id is in scope (e.g. a settings card header).
export function ProviderIcon({
  provider,
  size = 16,
  className,
  style,
}: {
  provider: string;
  size?: number;
  className?: string;
  style?: CSSProperties;
}) {
  return renderBrand(brandFromProvider(provider), { size, className, style });
}

// ModelIcon picks the badge by model name first (handles cases where a
// non-OpenAI model is routed through an OpenAI-compatible gateway), with
// the provider id as a fallback when the model name is generic.
export function ModelIcon({
  model,
  provider,
  size = 16,
  className,
  style,
}: {
  model: string;
  provider?: string;
  size?: number;
  className?: string;
  style?: CSSProperties;
}) {
  const brand = brandFromModel(model) ?? brandFromProvider(provider ?? '');
  return renderBrand(brand, { size, className, style });
}
