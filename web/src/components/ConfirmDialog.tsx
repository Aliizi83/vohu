import { useState } from "react"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { useLanguage } from "@/lib/i18n"

interface ConfirmOptions {
  title?: string
  description: string
  confirmLabel?: string
  cancelLabel?: string
  destructive?: boolean
}

interface PendingConfirm extends ConfirmOptions {
  resolve: (confirmed: boolean) => void
}

// A promise-based replacement for window.confirm() styled to match the
// rest of the app instead of the browser's unstyleable native dialog —
// call sites stay almost identical (`if (!(await confirm({...}))) return`
// instead of `if (!confirm("...")) return`).
export function useConfirm() {
  const { t } = useLanguage()
  const [pending, setPending] = useState<PendingConfirm | null>(null)

  function confirm(options: ConfirmOptions): Promise<boolean> {
    return new Promise((resolve) => {
      setPending({ ...options, resolve })
    })
  }

  function settle(confirmed: boolean) {
    pending?.resolve(confirmed)
    setPending(null)
  }

  const confirmDialog = (
    <AlertDialog open={pending !== null} onOpenChange={(open) => !open && settle(false)}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{pending?.title ?? t("common.confirmTitle")}</AlertDialogTitle>
          <AlertDialogDescription>{pending?.description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel onClick={() => settle(false)}>
            {pending?.cancelLabel ?? t("common.cancel")}
          </AlertDialogCancel>
          <AlertDialogAction
            variant={pending?.destructive === false ? "default" : "destructive"}
            onClick={() => settle(true)}
          >
            {pending?.confirmLabel ?? t("common.delete")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )

  return { confirm, confirmDialog }
}
