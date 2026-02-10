import cn from 'classnames'
import { IoArrowForwardSharp } from 'react-icons/io5'
import { Button } from '@/components/ui/button'
import { useStartStore } from '../../store/useStartStore'
import { useNavigate } from 'react-router-dom'
import { PiSpinnerGap } from 'react-icons/pi'
import { Input } from '@/components/ui/input'
import { useLetsEncryptInit } from '../../apis/letsencrypt'
import { useState } from 'react'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
    DialogClose,
} from "@/components/ui/dialog"

import { useLanguageStore } from '../../store/useLanguageStore'

export function LetsEncrypt() {
    const { setStep } = useStartStore()
    const [email, setEmail] = useState('')
    const [loading, setLoading] = useState(false)
    const { trigger: initLetsEncrypt, error: initError } = useLetsEncryptInit()
    const { t } = useLanguageStore()

    return (
        <>
            <div className='md:flex justify-between items-center mt-8 @xs/modal:mt-0'>
                <div>
                    <div className='text-xl @xs/modal:hidden'>{t('configure_ssl_le')}</div>
                    <div className='opacity-70 text-sm mt-2'>{t('ssl_le_desc')}</div>
                </div>
                <img src='/images/letsencrypt-logo-horizontal.svg' className='w-32 mt-3 md:mt-0' alt="Let's Encrypt" />
                {/* <div className="text-2xl font-bold opacity-50 @xs/modal:hidden">Let's Encrypt</div> */}
            </div>

            <div className='mt-8 space-y-4'>
                <div>
                    <label className='text-sm opacity-70 mb-1 block'>{t('email')}</label>
                    <Input
                        type='email'
                        placeholder={t('email_placeholder')}
                        className='bg-white'
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                    />
                    <div className='text-xs mt-1 opacity-50'>
                        {t('email_desc')}
                    </div>
                </div>
            </div>

            <div className='mt-8 flex justify-between'>
                <Dialog>
                    <DialogTrigger asChild>
                        <Button variant='destructive' className='cursor-pointer @xs/modal:hidden'>{t('skip_encryption')}</Button>
                    </DialogTrigger>
                    <DialogContent>
                        <DialogHeader>
                            <DialogTitle>{t('confirm_skip_encryption')}</DialogTitle>
                            <DialogDescription>
                                {t('skip_warning_desc')}
                            </DialogDescription>
                        </DialogHeader>
                        <DialogFooter>
                            <DialogClose asChild>
                                <Button variant="secondary">{t('cancel')}</Button>
                            </DialogClose>
                            <Button variant="destructive" onClick={() => setStep('select-user-type')}>{t('confirm_skip')}</Button>
                        </DialogFooter>
                    </DialogContent>
                </Dialog>
                <Button disabled={loading || !email} className='cursor-pointer' onClick={async () => {
                    try {
                        if (loading) return;
                        setLoading(true)
                        await initLetsEncrypt({ email })

                        // Wait for server restart and poll new domain
                        await new Promise(async resolve => {
                            while (true) {
                                try {
                                    // Try to fetch https version of the current host
                                    await fetch(`https://${window.location.hostname}:${window.location.port}`, { mode: 'no-cors' })
                                    // If no-cors fetch doesn't throw, it might mean connection established
                                    resolve(true)
                                    break;
                                } catch (e) {
                                    // ignore error and retry
                                }
                                await new Promise(resolve => setTimeout(resolve, 2000))
                            }
                        })
                        window.location.href = `https://${window.location.hostname}:${window.location.port}`
                    } finally {
                        setLoading(false)
                    }
                }}>{t('encrypt_my_connection')} {loading ? <PiSpinnerGap className='animate-spin' /> : <IoArrowForwardSharp className="rtl:rotate-180" />}</Button>
            </div>
            {initError && <div className='bg-red-800 text-white p-4 mt-8 rounded-lg'>
                {initError?.message || t('config_failed_check_logs')}
            </div>}
        </>
    )
}

