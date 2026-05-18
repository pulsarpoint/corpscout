import{s as i}from"./chunk-4N6VE7H7-BXN-OtmC.js";import{j as v}from"./jsx-runtime-D_zvdyIk.js";import{S as p,c as m,e as b}from"./utils-DnirJ8eA.js";/**
 * @license lucide-react v0.513.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const h=r=>r.replace(/([a-z0-9])([A-Z])/g,"$1-$2").toLowerCase(),x=r=>r.replace(/^([A-Z])|[\s-_]+(\w)/g,(e,t,a)=>a?a.toUpperCase():t.toLowerCase()),c=r=>{const e=x(r);return e.charAt(0).toUpperCase()+e.slice(1)},d=(...r)=>r.filter((e,t,a)=>!!e&&e.trim()!==""&&a.indexOf(e)===t).join(" ").trim(),w=r=>{for(const e in r)if(e.startsWith("aria-")||e==="role"||e==="title")return!0};/**
 * @license lucide-react v0.513.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */var y={xmlns:"http://www.w3.org/2000/svg",width:24,height:24,viewBox:"0 0 24 24",fill:"none",stroke:"currentColor",strokeWidth:2,strokeLinecap:"round",strokeLinejoin:"round"};/**
 * @license lucide-react v0.513.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const C=i.forwardRef(({color:r="currentColor",size:e=24,strokeWidth:t=2,absoluteStrokeWidth:a,className:s="",children:o,iconNode:u,...n},l)=>i.createElement("svg",{ref:l,...y,width:e,height:e,stroke:r,strokeWidth:a?Number(t)*24/Number(e):t,className:d("lucide",s),...!o&&!w(n)&&{"aria-hidden":"true"},...n},[...u.map(([f,g])=>i.createElement(f,g)),...Array.isArray(o)?o:[o]]));/**
 * @license lucide-react v0.513.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const L=(r,e)=>{const t=i.forwardRef(({className:a,...s},o)=>i.createElement(C,{ref:o,iconNode:e,className:d(`lucide-${h(c(r))}`,`lucide-${r}`,a),...s}));return t.displayName=c(r),t},k=b("inline-flex w-fit shrink-0 items-center justify-center gap-1 overflow-hidden rounded-full border border-transparent px-2 py-0.5 text-xs font-medium whitespace-nowrap transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&>svg]:pointer-events-none [&>svg]:size-3",{variants:{variant:{default:"bg-primary text-primary-foreground [a&]:hover:bg-primary/90",secondary:"bg-secondary text-secondary-foreground [a&]:hover:bg-secondary/90",destructive:"bg-destructive text-white focus-visible:ring-destructive/20 dark:bg-destructive/60 dark:focus-visible:ring-destructive/40 [a&]:hover:bg-destructive/90",outline:"border-border text-foreground [a&]:hover:bg-accent [a&]:hover:text-accent-foreground",ghost:"[a&]:hover:bg-accent [a&]:hover:text-accent-foreground",link:"text-primary underline-offset-4 [a&]:hover:underline"}},defaultVariants:{variant:"default"}});function B({className:r,variant:e="default",asChild:t=!1,...a}){const s=t?p:"span";return v.jsx(s,{"data-slot":"badge","data-variant":e,className:m(k({variant:e}),r),...a})}export{B,L as c};
